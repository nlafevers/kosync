package middleware

import (
	"log/slog"
	"net/http"
	"sync"

	"kosync/internal/api"
	"kosync/internal/database"

	"golang.org/x/time/rate"
)

const KOReaderMimeType = "application/vnd.koreader.v1+json"

// AuthMiddleware validates X-AUTH-USER and X-AUTH-KEY against the database.
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

		if !api.CheckPassword(hash, key) {
			slog.Warn("auth failure: invalid key", "username", username, "remote_addr", r.RemoteAddr)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
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
