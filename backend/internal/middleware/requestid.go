package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/diegoochoa/calculator-sezzle-api/internal/httpx"
)

// HeaderRequestID is the header read and echoed for request correlation.
const HeaderRequestID = "X-Request-Id"

const maxInboundRequestIDLength = 64

// RequestID attaches an id to every request and echoes it back, so a user's bug
// report ties directly to a log line.
//
// An inbound id is honoured only when it is short and alphanumeric: the value
// lands in logs and in a response header, and neither should carry attacker
// controlled newlines or unbounded length.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := sanitiseRequestID(r.Header.Get(HeaderRequestID))
		if id == "" {
			id = newRequestID()
		}

		w.Header().Set(HeaderRequestID, id)
		next.ServeHTTP(w, r.WithContext(httpx.WithRequestID(r.Context(), id)))
	})
}

func sanitiseRequestID(raw string) string {
	if raw == "" || len(raw) > maxInboundRequestIDLength {
		return ""
	}
	for _, r := range raw {
		isSafe := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_'
		if !isSafe {
			return ""
		}
	}
	return raw
}

func newRequestID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "unidentified"
	}
	return hex.EncodeToString(buf)
}
