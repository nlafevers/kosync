package api

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"kosync/internal/database"

	"golang.org/x/time/rate"
)

const KOReaderMimeType = "application/vnd.koreader.v1+json"

type responseWriter struct {
	http.ResponseWriter
	status    int
	size      int64
	errorBody []byte
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	if rw.status >= 500 {
		rw.errorBody = append(rw.errorBody, b...)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.size += int64(n)
	return n, err
}

// LoggingMiddleware logs HTTP requests and responses.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		requestID := generateRequestID()
		logger := slog.With(
			"method", r.Method,
			"path", r.URL.Path,
			"request_id", requestID,
		)

		ctx := context.WithValue(r.Context(), ContextKeyRequestID, requestID)
		ctx = context.WithValue(ctx, ContextKeyLogger, logger)
		r = r.WithContext(ctx)

		rw := &responseWriter{ResponseWriter: w}
		next.ServeHTTP(rw, r)

		duration := time.Since(start)
		user := GetUser(r.Context())

		fields := []any{
			"status_code", rw.status,
			"duration", duration,
			"remote_addr", r.RemoteAddr,
		}
		if user != "" {
			fields = append(fields, "user", user)
		}

		if rw.status >= 500 {
			fields = append(fields, "error_detail", string(rw.errorBody))
			logger.Error("server error", fields...)
		} else if rw.status >= 400 {
			logger.Warn("client error", fields...)
		} else {
			logger.Info("request completed", fields...)
		}

		logger.Debug("response diagnostics",
			"status_code", rw.status,
			"duration", duration,
			"response_size", rw.size,
			"user", user,
		)
	})
}

func generateRequestID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// AuthMiddleware validates X-AUTH-USER and X-AUTH-KEY against the database and stores the user in context.
func AuthMiddleware(storage *database.Storage, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := r.Header.Get("X-AUTH-USER")
		key := r.Header.Get("X-AUTH-KEY")

		if username == "" || key == "" {
			slog.Warn("missing auth headers", "remote_addr", r.RemoteAddr)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		hash, err := storage.GetUserHash(username)
		if err != nil {
			slog.Warn("auth failure: user not found", "username", username, "error", err, "remote_addr", r.RemoteAddr)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if !CheckPassword(hash, key) {
			slog.Warn("auth failure: invalid key", "username", username, "remote_addr", r.RemoteAddr)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Store user in context
		ctx := context.WithValue(r.Context(), ContextKeyUser, username)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AcceptMiddleware ensures the Accept header matches the required KOReader MIME type.
func AcceptMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != KOReaderMimeType {
			slog.Warn("invalid accept header", "accept", r.Header.Get("Accept"), "remote_addr", r.RemoteAddr)
			http.Error(w, "Not Acceptable", http.StatusNotAcceptable)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ContentTypeMiddleware ensures the response Content-Type is always set correctly.
func ContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", KOReaderMimeType)
		next.ServeHTTP(w, r)
	})
}

// IPRateLimiter handles rate limiting per IP address.
type IPRateLimiter struct {
	ips map[string]*rate.Limiter
	mu  sync.RWMutex
	r   rate.Limit
	b   int
}

func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	return &IPRateLimiter{
		ips: make(map[string]*rate.Limiter),
		r:   r,
		b:   b,
	}
}

func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.RLock()
	limiter, exists := i.ips[ip]
	i.mu.RUnlock()

	if !exists {
		i.mu.Lock()
		limiter = rate.NewLimiter(i.r, i.b)
		i.ips[ip] = limiter
		i.mu.Unlock()
	}

	return limiter
}

// RateLimitMiddleware applies rate limiting per IP.
func RateLimitMiddleware(limiter *IPRateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr // In production with a proxy, this might need X-Forwarded-For handling
		if !limiter.GetLimiter(ip).Allow() {
			slog.Warn("rate limit exceeded", "ip", ip)
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
