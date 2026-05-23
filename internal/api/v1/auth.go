package v1

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/t0mer/dnsmon/internal/api/apierr"
	"github.com/t0mer/dnsmon/internal/settings"
	"github.com/t0mer/dnsmon/internal/storage"
)

const sessionTTL = 7 * 24 * time.Hour

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type sessionResponse struct {
	Enabled       bool   `json:"enabled"`
	Authenticated bool   `json:"authenticated"`
	Username      string `json:"username,omitempty"`
}

// Login handles POST /api/v1/auth/login. On success it sets an HttpOnly
// session cookie.
func Login(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput, "Invalid request body.", nil)
			return
		}

		s, err := store.GetSettings(r.Context())
		if err != nil {
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal, "Failed to load settings.", nil)
			return
		}
		if !s.Auth.Enabled {
			apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput, "Authentication is not enabled.", nil)
			return
		}

		if req.Username != s.Auth.Username || !settings.VerifySecret(req.Password, s.Auth.PasswordHash) {
			apierr.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid username or password.", nil)
			return
		}

		// Lazily generate and persist a signing secret on first login.
		if s.SessionSecret == "" {
			secret, err := settings.GenerateToken()
			if err != nil {
				apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal, "Failed to start session.", nil)
				return
			}
			s.SessionSecret = secret
			if err := store.SaveSettings(r.Context(), s); err != nil {
				apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal, "Failed to persist session secret.", nil)
				return
			}
		}

		token := settings.SignSession(s.SessionSecret, s.Auth.Username, sessionTTL)
		http.SetCookie(w, &http.Cookie{
			Name:     settings.SessionCookie,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(sessionTTL),
		})
		apierr.WriteJSON(w, http.StatusOK, sessionResponse{Enabled: true, Authenticated: true, Username: s.Auth.Username})
	}
}

// Logout handles POST /api/v1/auth/logout, clearing the session cookie.
func Logout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     settings.SessionCookie,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
		})
		apierr.WriteJSON(w, http.StatusOK, sessionResponse{Authenticated: false})
	}
}

// Session handles GET /api/v1/auth/session, reporting whether auth is enabled
// and whether the caller is currently authenticated.
func Session(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := store.GetSettings(r.Context())
		if err != nil {
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal, "Failed to load settings.", nil)
			return
		}
		resp := sessionResponse{Enabled: s.Auth.Enabled}
		if s.Auth.Enabled {
			if c, err := r.Cookie(settings.SessionCookie); err == nil {
				if name, ok := settings.ParseSession(s.SessionSecret, c.Value); ok && name == s.Auth.Username {
					resp.Authenticated = true
					resp.Username = name
				}
			}
		}
		apierr.WriteJSON(w, http.StatusOK, resp)
	}
}
