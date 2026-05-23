package ratelimit

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Middleware returns a chi-compatible middleware that enforces the given rate limiter.
// If headers is true, X-RateLimit-* headers are set on every response.
func Middleware(anon *Limiter, headers bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := clientIP(r)

			if headers {
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", anon.ratePerMin))
				w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Minute).Unix()))
			}

			if !anon.Allow(key) {
				if headers {
					w.Header().Set("X-RateLimit-Remaining", "0")
					w.Header().Set("Retry-After", "60")
				}
				http.Error(w, `{"error":{"code":"RATE_LIMIT","message":"Too many requests."}}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		return host[:idx]
	}
	return host
}
