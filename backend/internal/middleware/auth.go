package middleware

import (
	"net/http"
	"strings"

	"github.com/diegoochoa/calculator-sezzle-api/internal/httpx"
)

// Verifier validates a bearer token and returns the client it identifies.
type Verifier interface {
	Verify(token string) (string, error)
}

// Authenticate rejects any request without a valid bearer token and puts the
// client id in the context for logging and rate limiting.
//
// Every failure returns the same body. Distinguishing "expired" from "bad
// signature" from "wrong issuer" tells an attacker which half of their guess
// was right.
func Authenticate(verifier Verifier) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r.Header.Get("Authorization"))
			if !ok {
				unauthorized(w, r, "Bearer token required")
				return
			}

			subject, err := verifier.Verify(token)
			if err != nil {
				unauthorized(w, r, "The token is not valid")
				return
			}

			next.ServeHTTP(w, r.WithContext(httpx.WithSubject(r.Context(), subject)))
		})
	}
}

// bearerToken extracts the credential from an Authorization header. The scheme
// is matched case-insensitively, as RFC 7235 requires.
func bearerToken(header string) (string, bool) {
	scheme, credential, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	credential = strings.TrimSpace(credential)
	if credential == "" {
		return "", false
	}
	return credential, true
}

func unauthorized(w http.ResponseWriter, r *http.Request, message string) {
	w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
	httpx.WriteError(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, message)
}
