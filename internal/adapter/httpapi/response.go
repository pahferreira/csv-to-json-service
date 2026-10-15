package httpapi

import (
	"encoding/json"
	"net/http"
)

// Pre-marshaled so the error path cannot itself fail to marshal.
var fallbackErrorBody = []byte(`{"error":{"code":"internal_error","message":"response encoding failed"}}`)

// statusResponse is the body of the two operational endpoints.
type statusResponse struct {
	Status string `json:"status"`
}

// errorResponse is ADR-0004's Request Rejection envelope, reused for 5xx.
type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func newErrorResponse(code, message string) errorResponse {
	return errorResponse{Error: errorBody{Code: code, Message: message}}
}

// writeJSON marshals before touching the writer, so an encoding failure is
// still recoverable into a 500 rather than a committed status with no body.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		body = fallbackErrorBody
		status = http.StatusInternalServerError
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	_, _ = w.Write(body) // Client is gone if this fails.
}
