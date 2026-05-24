package v1

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/t0mer/dnsmon/internal/api/apierr"
	"github.com/t0mer/dnsmon/internal/notify"
	"github.com/t0mer/dnsmon/internal/settings"
	"github.com/t0mer/dnsmon/internal/storage"
)

// settingsView is the redacted settings representation returned to clients.
// The admin password hash is never exposed; only whether one is set.
type settingsView struct {
	Auth struct {
		Enabled     bool   `json:"enabled"`
		Username    string `json:"username"`
		PasswordSet bool   `json:"password_set"`
	} `json:"auth"`
	Notifications     []settings.NotificationChannel `json:"notifications"`
	DisabledResolvers []string                       `json:"disabled_resolvers"`
}

func toView(s *settings.Settings) settingsView {
	var v settingsView
	v.Auth.Enabled = s.Auth.Enabled
	v.Auth.Username = s.Auth.Username
	v.Auth.PasswordSet = s.Auth.PasswordHash != ""
	v.Notifications = s.Notifications
	if v.Notifications == nil {
		v.Notifications = []settings.NotificationChannel{}
	}
	v.DisabledResolvers = s.DisabledResolvers
	if v.DisabledResolvers == nil {
		v.DisabledResolvers = []string{}
	}
	return v
}

// GetSettings handles GET /api/v1/settings.
func GetSettings(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := store.GetSettings(r.Context())
		if err != nil {
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal,
				"Failed to load settings.", nil)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, toView(s))
	}
}

type updateSettingsRequest struct {
	Auth struct {
		Enabled  bool   `json:"enabled"`
		Username string `json:"username"`
		Password string `json:"password"` // optional; replaces the stored password when non-empty
	} `json:"auth"`
	Notifications     []settings.NotificationChannel `json:"notifications"`
	DisabledResolvers []string                       `json:"disabled_resolvers"`
}

// UpdateSettings handles PUT /api/v1/settings.
func UpdateSettings(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req updateSettingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput,
				"Invalid request body.", nil)
			return
		}

		current, err := store.GetSettings(r.Context())
		if err != nil {
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal,
				"Failed to load settings.", nil)
			return
		}

		current.Auth.Enabled = req.Auth.Enabled
		current.Auth.Username = req.Auth.Username
		if req.Auth.Password != "" {
			hash, err := settings.HashSecret(req.Auth.Password)
			if err != nil {
				apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal,
					"Failed to hash password.", nil)
				return
			}
			current.Auth.PasswordHash = hash
		}
		if current.Auth.Enabled && (current.Auth.Username == "" || current.Auth.PasswordHash == "") {
			apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput,
				"A username and password are required to enable authentication.", nil)
			return
		}

		// Ensure every notification channel has a stable ID.
		channels := req.Notifications
		if channels == nil {
			channels = []settings.NotificationChannel{}
		}
		for i := range channels {
			if channels[i].ID == "" {
				channels[i].ID = settings.NewID()
			}
		}
		current.Notifications = channels

		if req.DisabledResolvers == nil {
			current.DisabledResolvers = []string{}
		} else {
			current.DisabledResolvers = req.DisabledResolvers
		}

		if err := store.SaveSettings(r.Context(), current); err != nil {
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal,
				"Failed to save settings.", nil)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, toView(current))
	}
}

// ListTokens handles GET /api/v1/settings/tokens.
func ListTokens(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokens, err := store.ListAPITokens(r.Context())
		if err != nil {
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal,
				"Failed to list API tokens.", nil)
			return
		}
		if tokens == nil {
			tokens = []*settings.APIToken{}
		}
		apierr.WriteJSON(w, http.StatusOK, tokens)
	}
}

type createTokenRequest struct {
	Name string `json:"name" validate:"required"`
}

type createTokenResponse struct {
	*settings.APIToken
	Token string `json:"token"` // raw secret, shown only once
}

// CreateToken handles POST /api/v1/settings/tokens.
func CreateToken(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput,
				"Invalid request body.", nil)
			return
		}
		if err := validate.Struct(req); err != nil {
			apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput,
				err.Error(), nil)
			return
		}

		raw, err := settings.GenerateToken()
		if err != nil {
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal,
				"Failed to generate token.", nil)
			return
		}
		hash, err := settings.HashSecret(raw)
		if err != nil {
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal,
				"Failed to hash token.", nil)
			return
		}

		tok := &settings.APIToken{
			ID:        settings.NewID(),
			Name:      req.Name,
			Hash:      hash,
			Prefix:    raw[:8],
			CreatedAt: time.Now().UTC(),
		}
		if err := store.CreateAPIToken(r.Context(), tok); err != nil {
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal,
				"Failed to create token.", nil)
			return
		}
		apierr.WriteJSON(w, http.StatusCreated, createTokenResponse{APIToken: tok, Token: raw})
	}
}

// DeleteToken handles DELETE /api/v1/settings/tokens/{id}.
func DeleteToken(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := store.DeleteAPIToken(r.Context(), id); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				apierr.WriteError(w, r, http.StatusNotFound, apierr.ErrCodeNotFound, "Token not found.", nil)
				return
			}
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal,
				"Failed to delete token.", nil)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ListSchedules handles GET /api/v1/settings/schedules.
func ListSchedules(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := store.ListSchedules(r.Context())
		if err != nil {
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal,
				"Failed to list schedules.", nil)
			return
		}
		if list == nil {
			list = []*settings.Schedule{}
		}
		apierr.WriteJSON(w, http.StatusOK, list)
	}
}

type scheduleRequest struct {
	Name        string   `json:"name" validate:"required"`
	Domain      string   `json:"domain" validate:"required"`
	Type        string   `json:"type" validate:"required"`
	Resolvers   []string `json:"resolvers"`
	IntervalSec int      `json:"interval_sec"`
	Enabled     bool     `json:"enabled"`
}

func (req scheduleRequest) validate(w http.ResponseWriter, r *http.Request) bool {
	if err := validate.Struct(req); err != nil {
		apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput, err.Error(), nil)
		return false
	}
	if !domainRE.MatchString(req.Domain) {
		apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidDomain, "Invalid domain name.", nil)
		return false
	}
	return true
}

// CreateSchedule handles POST /api/v1/settings/schedules.
func CreateSchedule(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req scheduleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput,
				"Invalid request body.", nil)
			return
		}
		if !req.validate(w, r) {
			return
		}

		now := time.Now().UTC()
		sc := &settings.Schedule{
			ID:          settings.NewID(),
			Name:        req.Name,
			Domain:      req.Domain,
			Type:        req.Type,
			Resolvers:   req.Resolvers,
			IntervalSec: req.IntervalSec,
			Enabled:     req.Enabled,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := store.SaveSchedule(r.Context(), sc); err != nil {
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal,
				"Failed to create schedule.", nil)
			return
		}
		apierr.WriteJSON(w, http.StatusCreated, sc)
	}
}

// UpdateSchedule handles PUT /api/v1/settings/schedules/{id}.
func UpdateSchedule(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		var req scheduleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput,
				"Invalid request body.", nil)
			return
		}
		if !req.validate(w, r) {
			return
		}

		existing, err := findSchedule(store, r, id)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				apierr.WriteError(w, r, http.StatusNotFound, apierr.ErrCodeNotFound, "Schedule not found.", nil)
				return
			}
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal,
				"Failed to load schedule.", nil)
			return
		}

		existing.Name = req.Name
		existing.Domain = req.Domain
		existing.Type = req.Type
		existing.Resolvers = req.Resolvers
		existing.IntervalSec = req.IntervalSec
		existing.Enabled = req.Enabled
		existing.UpdatedAt = time.Now().UTC()

		if err := store.SaveSchedule(r.Context(), existing); err != nil {
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal,
				"Failed to update schedule.", nil)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, existing)
	}
}

// DeleteSchedule handles DELETE /api/v1/settings/schedules/{id}.
func DeleteSchedule(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := store.DeleteSchedule(r.Context(), id); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				apierr.WriteError(w, r, http.StatusNotFound, apierr.ErrCodeNotFound, "Schedule not found.", nil)
				return
			}
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal,
				"Failed to delete schedule.", nil)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func findSchedule(store storage.Storage, r *http.Request, id string) (*settings.Schedule, error) {
	list, err := store.ListSchedules(r.Context())
	if err != nil {
		return nil, err
	}
	for _, sc := range list {
		if sc.ID == id {
			return sc, nil
		}
	}
	return nil, storage.ErrNotFound
}

// TestNotification handles POST /api/v1/settings/notifications/test. It sends a
// test message to the channel described in the request body.
func TestNotification(sender *notify.Sender) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var ch settings.NotificationChannel
		if err := json.NewDecoder(r.Body).Decode(&ch); err != nil {
			apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput, "Invalid request body.", nil)
			return
		}
		if ch.Type == "" {
			apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput, "Channel type is required.", nil)
			return
		}

		if err := sender.Send(r.Context(), ch, "dnsmon test notification — your channel is configured correctly."); err != nil {
			apierr.WriteError(w, r, http.StatusBadGateway, "NOTIFY_FAILED", err.Error(), nil)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, map[string]string{"status": "sent"})
	}
}
