package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tomerklein/gdns/internal/api/apierr"
	"github.com/tomerklein/gdns/internal/export"
	"github.com/tomerklein/gdns/internal/storage"
)

// ExportCheck handles GET /api/v1/check/{id}/export?format=json|csv|png|svg|pdf.
func ExportCheck(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		format := r.URL.Query().Get("format")
		if format == "" {
			format = "json"
		}

		check, err := store.GetCheck(r.Context(), id)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				apierr.WriteError(w, r, http.StatusNotFound, apierr.ErrCodeNotFound,
					"Check not found.", nil)
				return
			}
			apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal,
				"Failed to retrieve check.", nil)
			return
		}

		filename := fmt.Sprintf("gdns-%s-%s.%s", check.Name, check.Type, format)

		switch format {
		case "json":
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			_ = enc.Encode(check)

		case "csv":
			w.Header().Set("Content-Type", "text/csv")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
			if err := export.CSV(w, check); err != nil {
				apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal,
					"Failed to generate CSV.", nil)
			}

		case "png":
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
			if err := export.PNG(w, check); err != nil {
				apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal,
					"Failed to generate PNG.", nil)
			}

		case "svg":
			w.Header().Set("Content-Type", "image/svg+xml")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
			if err := export.SVG(w, check); err != nil {
				apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal,
					"Failed to generate SVG.", nil)
			}

		case "pdf":
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
			if err := export.PDF(w, check); err != nil {
				apierr.WriteError(w, r, http.StatusInternalServerError, apierr.ErrCodeInternal,
					"Failed to generate PDF.", nil)
			}

		default:
			apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput,
				fmt.Sprintf("Unsupported format: %s. Use json, csv, png, svg, or pdf.", format), nil)
		}
	}
}
