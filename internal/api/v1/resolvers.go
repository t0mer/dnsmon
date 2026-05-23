package v1

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/t0mer/dnsmon/internal/api/apierr"
	"github.com/t0mer/dnsmon/internal/resolvers"
)

// ListResolvers handles GET /api/v1/resolvers.
// Supports ?country=US&q=google query params.
func ListResolvers(registry *resolvers.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		country := r.URL.Query().Get("country")
		q := r.URL.Query().Get("q")

		var countries []string
		if country != "" {
			for _, c := range strings.Split(country, ",") {
				if c = strings.TrimSpace(c); c != "" {
					countries = append(countries, c)
				}
			}
		}

		list := registry.Filter(countries, q)
		apierr.WriteJSON(w, http.StatusOK, list)
	}
}

// GetResolver handles GET /api/v1/resolvers/{id}.
func GetResolver(registry *resolvers.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		res, ok := registry.Get(id)
		if !ok {
			apierr.WriteError(w, r, http.StatusNotFound, apierr.ErrCodeNotFound,
				"Resolver not found.", nil)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, res)
	}
}
