package api

import (
	"net/http"

	"github.com/tomerklein/gdns/internal/api/apierr"
)

// Re-export constants for callers that import api directly.
const (
	ErrCodeInvalidInput      = apierr.ErrCodeInvalidInput
	ErrCodeNotFound          = apierr.ErrCodeNotFound
	ErrCodeRateLimit         = apierr.ErrCodeRateLimit
	ErrCodeInternal          = apierr.ErrCodeInternal
	ErrCodeInvalidRecordType = apierr.ErrCodeInvalidRecordType
	ErrCodeInvalidDomain     = apierr.ErrCodeInvalidDomain
)

// ErrorResponse is the standard JSON error envelope.
type ErrorResponse = apierr.ErrorResponse

// ErrorDetail holds the error code, message, and optional details.
type ErrorDetail = apierr.ErrorDetail

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string, details interface{}) {
	apierr.WriteError(w, r, status, code, message, details)
}

// WriteJSON writes v as a JSON response with the given HTTP status code.
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	apierr.WriteJSON(w, status, v)
}
