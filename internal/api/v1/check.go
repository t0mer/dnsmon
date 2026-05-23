package v1

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/t0mer/dnsmon/internal/api/apierr"
	"github.com/t0mer/dnsmon/internal/checker"
	"github.com/t0mer/dnsmon/internal/dnsclient"
	"github.com/t0mer/dnsmon/internal/storage"
)

var (
	validate  = validator.New()
	domainRE  = regexp.MustCompile(`^[a-zA-Z0-9._\-]+$`)
)

// CheckRequest is the request body for POST /api/v1/check.
type CheckRequest struct {
	Name            string               `json:"name" validate:"required"`
	Type            string               `json:"type" validate:"required"`
	Resolvers       []string             `json:"resolvers"`
	CustomResolvers []dnsclient.Resolver `json:"custom_resolvers"`
	Save            bool                 `json:"save"`
}

// PostCheck handles POST /api/v1/check.
func PostCheck(chkr *checker.Checker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CheckRequest
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

		if !domainRE.MatchString(req.Name) {
			apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidDomain,
				"Invalid domain name.", nil)
			return
		}

		check, err := chkr.Check(r.Context(), checker.CheckRequest{
			Name:            req.Name,
			Type:            req.Type,
			ResolverIDs:     req.Resolvers,
			CustomResolvers: req.CustomResolvers,
			Save:            req.Save,
		})
		if err != nil {
			apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput,
				err.Error(), nil)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, check)
	}
}

// GetCheck handles GET /api/v1/check/{id}.
func GetCheck(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
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
		apierr.WriteJSON(w, http.StatusOK, check)
	}
}
