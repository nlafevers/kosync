package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nlafevers/kosync/internal/database"

	"golang.org/x/time/rate"
)

// KOReaderMimeType is the MIME type the KOReader client sends in its Accept
// header on every request; AcceptMiddleware requires it.
const KOReaderMimeType = "application/vnd.koreader.v1+json"

// ResponseMimeType is the Content-Type we send on responses. It must be plain
// "application/json" rather than KOReaderMimeType: KOReader's Spore client only
// decodes a response body into an object when the Content-Type passes its JSON
// check. Before koreader-base's 2026-02 "detect more json content types" patch,
// that check was a literal substring search for "application/json", which
// "application/vnd.koreader.v1+json" fails — leaving the body an undecoded
// string so the sync plugin sees no progress ("No progress found for this
// document."). Plain "application/json" is accepted by both the old substring
// check and the newer pattern, so it is compatible with all client versions.
const ResponseMimeType = "application/json"

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
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// AuthMiddleware validates X-AUTH-USER and X-AUTH-KEY against the database and stores the user in context.
func AuthMiddleware(storage *database.Storage, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := GetLogger(r.Context())
		username := r.Header.Get("X-AUTH-USER")
		key := r.Header.Get("X-AUTH-KEY")

		if username == "" || key == "" {
			log.Warn("missing auth headers", "remote_addr", r.RemoteAddr, "source", "API")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		hash, err := storage.GetUserHash(username)
		if err != nil {
			log.Warn("auth failure: user not found", "username", username, "error", err, "remote_addr", r.RemoteAddr, "source", "API")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if !CheckPassword(hash, key) {
			log.Warn("auth failure: invalid key", "username", username, "remote_addr", r.RemoteAddr, "source", "API")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		log.Debug("auth success", "username", username, "source", "API")
		// Store user in context
		ctx := context.WithValue(r.Context(), ContextKeyUser, username)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AcceptMiddleware ensures the Accept header matches the required KOReader MIME type.
func AcceptMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != KOReaderMimeType {
			GetLogger(r.Context()).Warn("invalid accept header", "accept", r.Header.Get("Accept"), "remote_addr", r.RemoteAddr, "source", "API")
			http.Error(w, "Not Acceptable", http.StatusNotAcceptable)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ContentTypeMiddleware ensures the response Content-Type is always set correctly.
func ContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ResponseMimeType)
		next.ServeHTTP(w, r)
	})
}

// IPRateLimiter handles rate limiting per IP address.
type IPRateLimiter struct {
	ips        map[string]*rate.Limiter
	mu         sync.RWMutex
	r          rate.Limit
	b          int
	trustProxy bool
}

func NewIPRateLimiter(r rate.Limit, b int, trustProxy bool) *IPRateLimiter {
	return &IPRateLimiter{
		ips:        make(map[string]*rate.Limiter),
		r:          r,
		b:          b,
		trustProxy: trustProxy,
	}
}

func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.RLock()
	limiter, exists := i.ips[ip]
	i.mu.RUnlock()

	if !exists {
		i.mu.Lock()
		// Double-check after acquiring write lock.
		if limiter, exists = i.ips[ip]; !exists {
			limiter = rate.NewLimiter(i.r, i.b)
			i.ips[ip] = limiter
		}
		i.mu.Unlock()
	}

	return limiter
}

// clientIP returns the client's IP address. If trustProxy is true and the
// X-Forwarded-For header is present, the first (leftmost) address is used.
// Otherwise the IP is taken from r.RemoteAddr.
func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			ip := strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])
			if ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// RateLimitMiddleware applies rate limiting per IP.
func RateLimitMiddleware(limiter *IPRateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r, limiter.trustProxy)
		if !limiter.GetLimiter(ip).Allow() {
			GetLogger(r.Context()).Warn("rate limit exceeded", "ip", ip, "source", "API")
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
