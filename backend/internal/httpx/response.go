// Package httpx holds the pieces shared by the handler and middleware layers:
// the JSON response envelope, transport-level error codes and the request-id
// context. Keeping them here lets middleware emit the same envelope as handlers
// without an import cycle.
package httpx

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
)

// Transport-level error codes. Codes raised by the calculation engine itself
// live in internal/calc.
const (
	CodeInvalidRequest   = "INVALID_REQUEST"
	CodeUnauthorized     = "UNAUTHORIZED"
	CodeForbidden        = "FORBIDDEN"
	CodeNotFound         = "NOT_FOUND"
	CodeMethodNotAllowed = "METHOD_NOT_ALLOWED"
	CodePayloadTooLarge  = "PAYLOAD_TOO_LARGE"
	CodeRateLimited      = "RATE_LIMITED"
	CodeTimeout          = "TIMEOUT"
	CodeInternal         = "INTERNAL_ERROR"
)

// ErrorBody describes one failure. Position, when present, is the 0-based rune
// offset in the submitted expression, so a client can underline the problem.
type ErrorBody struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Position *int   `json:"position,omitempty"`
}

// ErrorResponse is the envelope every failure uses, whatever the status.
type ErrorResponse struct {
	Error     ErrorBody `json:"error"`
	RequestID string    `json:"requestId,omitempty"`
}

// WriteJSON serialises payload before touching the ResponseWriter, so an
// encoding failure produces a clean 500 instead of a truncated body behind an
// already-sent 200.
func WriteJSON(w http.ResponseWriter, status int, payload any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		slog.Error("encoding response failed", "error", err)
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Response could not be encoded"}}`,
			http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if _, err := w.Write(buf.Bytes()); err != nil {
		slog.Debug("writing response failed", "error", err)
	}
}

// WriteError emits the standard envelope, tagged with the request id so a user
// report can be traced straight to a log line.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeError(w, r, status, ErrorBody{Code: code, Message: message})
}

// WriteErrorAt is WriteError with the offending offset in the expression.
func WriteErrorAt(w http.ResponseWriter, r *http.Request, status int, code, message string, position int) {
	writeError(w, r, status, ErrorBody{Code: code, Message: message, Position: &position})
}

func writeError(w http.ResponseWriter, r *http.Request, status int, body ErrorBody) {
	WriteJSON(w, status, ErrorResponse{Error: body, RequestID: RequestID(r.Context())})
}
