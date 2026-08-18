package httpapi

import (
	"net/http"
	"time"

	"github.com/diegoochoa/calculator-sezzle-api/internal/httpx"
)

type tokenRequest struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

type tokenResponse struct {
	Token     string    `json:"token"`
	TokenType string    `json:"tokenType"`
	ExpiresIn int       `json:"expiresIn"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// handleToken exchanges client credentials for a bearer token.
//
// Every failure — unknown client, wrong secret, missing field — returns the same
// 401 and the same message. Anything more specific tells an attacker which half
// of their guess was right, and this route is the obvious brute-force target.
func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	var request tokenRequest
	if !decodeJSON(w, r, &request) {
		return
	}

	if request.ClientID == "" || request.ClientSecret == "" ||
		!s.clients.Authenticate(request.ClientID, request.ClientSecret) {
		s.logger.Warn("token request rejected",
			"clientId", request.ClientID,
			"requestId", httpx.RequestID(r.Context()),
		)
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_client"`)
		httpx.WriteError(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized,
			"The client credentials are not valid")
		return
	}

	token, err := s.issuer.Issue(request.ClientID)
	if err != nil {
		s.logger.Error("issuing a token failed",
			"error", err,
			"requestId", httpx.RequestID(r.Context()),
		)
		httpx.WriteError(w, r, http.StatusInternalServerError, httpx.CodeInternal,
			"The token could not be issued")
		return
	}

	// Credentials must never be cached by a proxy or the browser.
	w.Header().Set("Cache-Control", "no-store")

	httpx.WriteJSON(w, http.StatusOK, tokenResponse{
		Token:     token.Value,
		TokenType: "Bearer",
		ExpiresIn: token.ExpiresIn,
		ExpiresAt: token.ExpiresAt.UTC(),
	})
}
