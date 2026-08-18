package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/diegoochoa/calculator-sezzle-api/internal/httpx"
)

// decodeJSON reads a request body into target. It writes the failure itself and
// reports whether the caller may continue.
//
// Unknown fields are rejected: a client sending {"expresion": "1+1"} has a bug,
// and silently evaluating an empty expression instead of saying so wastes an
// afternoon. Content-Type is deliberately not enforced, so `curl -d` works
// without ceremony.
func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		writeDecodeError(w, r, err)
		return false
	}

	// A second value in the body means the client sent something we would only
	// half-honour.
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.CodeInvalidRequest,
			"The body must contain exactly one JSON object")
		return false
	}

	return true
}

func writeDecodeError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		syntaxError   *json.SyntaxError
		typeError     *json.UnmarshalTypeError
		maxBytesError *http.MaxBytesError
	)

	switch {
	case errors.As(err, &maxBytesError):
		httpx.WriteError(w, r, http.StatusRequestEntityTooLarge, httpx.CodePayloadTooLarge,
			fmt.Sprintf("The request body must not exceed %d bytes", maxBytesError.Limit))

	case errors.Is(err, io.EOF):
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.CodeInvalidRequest,
			"The request body is empty")

	case errors.As(err, &syntaxError):
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.CodeInvalidRequest,
			fmt.Sprintf("The body is not valid JSON (at byte %d)", syntaxError.Offset))

	case errors.Is(err, io.ErrUnexpectedEOF):
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.CodeInvalidRequest,
			"The body ends in the middle of a JSON value")

	case errors.As(err, &typeError):
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.CodeInvalidRequest,
			fmt.Sprintf("Field %q must be a %s", typeError.Field, typeError.Type))

	case strings.HasPrefix(err.Error(), "json: unknown field "):
		field := strings.TrimPrefix(err.Error(), "json: unknown field ")
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.CodeInvalidRequest,
			fmt.Sprintf("Unknown field %s", field))

	default:
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.CodeInvalidRequest,
			"The request body could not be read")
	}
}
