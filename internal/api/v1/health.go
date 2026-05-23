package v1

import (
	"net/http"

	"github.com/t0mer/dnsmon/internal/api/apierr"
	"github.com/t0mer/dnsmon/internal/storage"
	"github.com/t0mer/dnsmon/internal/version"
)

// Health returns 200 with {"status":"ok"}.
func Health(w http.ResponseWriter, r *http.Request) {
	apierr.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readyz returns 200 when storage is reachable, 503 otherwise.
func Readyz(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := store.ListChecks(r.Context(), 1, 0)
		if err != nil {
			apierr.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable",
				"error":  err.Error(),
			})
			return
		}
		apierr.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// Version returns the build version information.
func Version(w http.ResponseWriter, r *http.Request) {
	apierr.WriteJSON(w, http.StatusOK, version.BuildInfo())
}
