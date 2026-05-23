package apierr

import (
	"encoding/json"
	"net/http"
)

const (
	ErrCodeInvalidInput      = "INVALID_INPUT"
	ErrCodeNotFound          = "NOT_FOUND"
	ErrCodeRateLimit         = "RATE_LIMIT"
	ErrCodeInternal          = "INTERNAL_ERROR"
	ErrCodeInvalidRecordType = "INVALID_RECORD_TYPE"
	ErrCodeInvalidDomain     = "INVALID_DOMAIN"
)

// ErrorResponse is the standard JSON error envelope.
type ErrorResponse struct {
	Error     ErrorDetail `json:"error"`
	RequestID string      `json:"request_id"`
}

// ErrorDetail holds the error code, message, and optional details.
type ErrorDetail struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string, details interface{}) {
	resp := ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
		RequestID: requestID(r),
	}
	WriteJSON(w, status, resp)
}

// WriteJSON writes v as a JSON response with the given HTTP status code.
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func requestID(r *http.Request) string {
	return r.Header.Get("X-Request-ID")
}
