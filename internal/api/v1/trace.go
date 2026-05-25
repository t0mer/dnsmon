package v1

import (
	"encoding/json"
	"net/http"

	"github.com/t0mer/dnsmon/internal/api/apierr"
	"github.com/t0mer/dnsmon/internal/checker"
)

// TraceRequest is the request body for POST /api/v1/trace.
type TraceRequest struct {
	Name string `json:"name" validate:"required"`
	Type string `json:"type" validate:"required"`
}

// PostTrace handles POST /api/v1/trace.
func PostTrace(chkr *checker.Checker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req TraceRequest
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

		result, err := chkr.Trace(r.Context(), req.Name, req.Type)
		if err != nil {
			apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput,
				err.Error(), nil)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, result)
	}
}
