package v1

import (
	"encoding/json"
	"net/http"

	"github.com/tomerklein/gdns/internal/api/apierr"
	"github.com/tomerklein/gdns/internal/checker"
)

// LookupRequest is the request body for POST /api/v1/lookup.
type LookupRequest struct {
	Name     string `json:"name" validate:"required"`
	Type     string `json:"type" validate:"required"`
	Resolver string `json:"resolver"`
	Protocol string `json:"protocol"`
}

// PostLookup handles POST /api/v1/lookup.
func PostLookup(chkr *checker.Checker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req LookupRequest
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

		result, err := chkr.Lookup(r.Context(), checker.LookupRequest{
			Name:     req.Name,
			Type:     req.Type,
			Resolver: req.Resolver,
			Protocol: req.Protocol,
		})
		if err != nil {
			apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput,
				err.Error(), nil)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, result)
	}
}
