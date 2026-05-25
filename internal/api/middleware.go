package api

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/t0mer/dnsmon/internal/api/apierr"
	"github.com/t0mer/dnsmon/internal/settings"
	"github.com/t0mer/dnsmon/internal/storage"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// RequestID sets a unique X-Request-ID header on each request and stores it in context.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.status = code
	rr.ResponseWriter.WriteHeader(code)
}

// Unwrap exposes the underlying ResponseWriter so http.ResponseController can
// reach capabilities like Hijack/Flush.
func (rr *responseRecorder) Unwrap() http.ResponseWriter {
	return rr.ResponseWriter
}

// Hijack lets WebSocket upgrades work through the logging wrapper.
func (rr *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := rr.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
	}
	return hj.Hijack()
}

// Logger returns a middleware that logs each request using slog.
func Logger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			log.InfoContext(r.Context(), "http request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Duration("duration", time.Since(start)),
				slog.String("request_id", r.Header.Get("X-Request-ID")),
			)
		})
	}
}

// Recovery returns a middleware that recovers from panics and logs them.
func Recovery(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.ErrorContext(r.Context(), "panic recovered",
						slog.Any("panic", rec),
						slog.String("path", r.URL.Path),
					)
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"error":{"code":"INTERNAL_ERROR","message":"Internal server error."}}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth gates access when UI authentication is enabled in settings. When
// disabled, requests pass through untouched. When enabled, a valid session
// cookie is required; unauthenticated API requests get 401 (apiMode=true) while
// unauthenticated page requests are redirected to /login (apiMode=false).
func RequireAuth(store storage.Storage, apiMode bool) func(http.Handler) http.Handler {
	deny := func(w http.ResponseWriter, r *http.Request) {
		if apiMode {
			apierr.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required.", nil)
			return
		}
		http.Redirect(w, r, "/login", http.StatusFound)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s, err := store.GetSettings(r.Context())
			if err != nil {
				deny(w, r)
				return
			}
			if !s.Auth.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			c, err := r.Cookie(settings.SessionCookie)
			if err != nil {
				deny(w, r)
				return
			}
			name, ok := settings.ParseSession(s.SessionSecret, c.Value)
			if !ok || name != s.Auth.Username {
				deny(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CORS returns a middleware that adds CORS headers for API routes. allowedOrigin
// should be the server's base URL (e.g. "https://dnsmon.example.com"). When empty
// it falls back to "*" so that unconfigured installs remain functional.
func CORS(allowedOrigin string) func(http.Handler) http.Handler {
	origin := allowedOrigin
	if origin == "" {
		origin = "*"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SecureHeaders returns a middleware that sets defensive HTTP response headers on
// every response to reduce XSS, clickjacking, and MIME-sniffing attack surface.
func SecureHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("Content-Security-Policy",
				"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'")
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
			next.ServeHTTP(w, r)
		})
	}
}
