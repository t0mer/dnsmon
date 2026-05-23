package v1

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tomerklein/gdns/internal/api/apierr"
	"github.com/tomerklein/gdns/internal/dnsclient"
	"github.com/tomerklein/gdns/internal/storage"
)

// ListHistory handles GET /api/v1/history.
func ListHistory(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 20
		offset := 0

		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
				limit = n
			}
		}
		if o := r.URL.Query().Get("offset"); o != "" {
			if n, err := strconv.Atoi(o); err == nil && n >= 0 {
				offset = n
			}
		}

		checks, err := store.ListChecks(r.Context(), limit, offset)
		if err != nil {
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal,
				"Failed to retrieve history.", nil)
			return
		}

		if checks == nil {
			checks = []*dnsclient.Check{}
		}

		apierr.WriteJSON(w, http.StatusOK, checks)
	}
}

// DeleteHistory handles DELETE /api/v1/history/{id}.
func DeleteHistory(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := store.DeleteCheck(r.Context(), id); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				apierr.WriteError(w, r, http.StatusNotFound, apierr.ErrCodeNotFound,
					"Check not found.", nil)
				return
			}
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal,
				"Failed to delete check.", nil)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
