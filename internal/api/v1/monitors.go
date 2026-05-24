package v1

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/t0mer/dnsmon/internal/api/apierr"
	"github.com/t0mer/dnsmon/internal/dnsclient"
	"github.com/t0mer/dnsmon/internal/settings"
	"github.com/t0mer/dnsmon/internal/storage"
)

type monitorRequest struct {
	Name        string   `json:"name"`
	Type        string   `json:"type" validate:"required"`
	FQDN        string   `json:"fqdn" validate:"required"`
	RecordType  string   `json:"record_type" validate:"required"`
	Expected    []string `json:"expected"`
	SchedulerID string   `json:"scheduler_id"`
	ChannelID   string   `json:"channel_id"`
	Enabled     bool     `json:"enabled"`
}

func (req *monitorRequest) validate(w http.ResponseWriter, r *http.Request) bool {
	if err := validate.Struct(req); err != nil {
		apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput, err.Error(), nil)
		return false
	}
	if req.Type != settings.MonitorPropagation && req.Type != settings.MonitorChange {
		apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput,
			"type must be 'propagation' or 'change'.", nil)
		return false
	}
	if !domainRE.MatchString(req.FQDN) {
		apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidDomain, "Invalid FQDN.", nil)
		return false
	}
	if _, ok := dnsclient.ParseType(req.RecordType); !ok {
		apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidRecordType,
			"Unsupported record type.", nil)
		return false
	}
	if req.Type == settings.MonitorPropagation && len(cleanValues(req.Expected)) == 0 {
		apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput,
			"Propagation monitors require at least one expected value.", nil)
		return false
	}
	return true
}

func cleanValues(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// ListMonitors handles GET /api/v1/settings/monitors.
func ListMonitors(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := store.ListMonitors(r.Context())
		if err != nil {
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal, "Failed to list monitors.", nil)
			return
		}
		if list == nil {
			list = []*settings.Monitor{}
		}
		apierr.WriteJSON(w, http.StatusOK, list)
	}
}

// CreateMonitor handles POST /api/v1/settings/monitors.
func CreateMonitor(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req monitorRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput, "Invalid request body.", nil)
			return
		}
		if !req.validate(w, r) {
			return
		}

		now := time.Now().UTC()
		m := &settings.Monitor{
			ID:          settings.NewID(),
			Name:        strings.TrimSpace(req.Name),
			Type:        req.Type,
			FQDN:        strings.TrimSpace(req.FQDN),
			RecordType:  req.RecordType,
			Expected:    cleanValues(req.Expected),
			SchedulerID: req.SchedulerID,
			ChannelID:   req.ChannelID,
			Enabled:     req.Enabled,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if m.Name == "" {
			m.Name = m.FQDN
		}
		if err := store.SaveMonitor(r.Context(), m); err != nil {
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal, "Failed to create monitor.", nil)
			return
		}

		// Seed the changelog with the initial baseline/expected snapshot.
		_ = store.AppendMonitorEvent(r.Context(), &settings.MonitorEvent{
			ID:        settings.NewID(),
			MonitorID: m.ID,
			Timestamp: now,
			Status:    settings.MonitorStatusCreated,
			Observed:  m.Expected,
			Message:   "Monitor created.",
		})

		apierr.WriteJSON(w, http.StatusCreated, m)
	}
}

// UpdateMonitor handles PUT /api/v1/settings/monitors/{id}.
func UpdateMonitor(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		var req monitorRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput, "Invalid request body.", nil)
			return
		}
		if !req.validate(w, r) {
			return
		}

		existing, err := store.GetMonitor(r.Context(), id)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				apierr.WriteError(w, r, http.StatusNotFound, apierr.ErrCodeNotFound, "Monitor not found.", nil)
				return
			}
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal, "Failed to load monitor.", nil)
			return
		}

		existing.Name = strings.TrimSpace(req.Name)
		if existing.Name == "" {
			existing.Name = strings.TrimSpace(req.FQDN)
		}
		existing.Type = req.Type
		existing.FQDN = strings.TrimSpace(req.FQDN)
		existing.RecordType = req.RecordType
		existing.Expected = cleanValues(req.Expected)
		existing.SchedulerID = req.SchedulerID
		existing.ChannelID = req.ChannelID
		existing.Enabled = req.Enabled
		existing.UpdatedAt = time.Now().UTC()

		if err := store.SaveMonitor(r.Context(), existing); err != nil {
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal, "Failed to update monitor.", nil)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, existing)
	}
}

// DeleteMonitor handles DELETE /api/v1/settings/monitors/{id}.
func DeleteMonitor(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := store.DeleteMonitor(r.Context(), id); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				apierr.WriteError(w, r, http.StatusNotFound, apierr.ErrCodeNotFound, "Monitor not found.", nil)
				return
			}
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal, "Failed to delete monitor.", nil)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// MonitorHistory handles GET /api/v1/settings/monitors/{id}/history.
func MonitorHistory(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		limit := 100
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 {
				limit = n
			}
		}
		events, err := store.ListMonitorEvents(r.Context(), id, limit)
		if err != nil {
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal, "Failed to load monitor history.", nil)
			return
		}
		if events == nil {
			events = []*settings.MonitorEvent{}
		}
		apierr.WriteJSON(w, http.StatusOK, events)
	}
}
