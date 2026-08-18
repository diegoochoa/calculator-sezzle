// Package httpapi holds the HTTP handlers and routing. It owns request
// decoding, validation and status mapping; all arithmetic lives in
// internal/calc.
package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/diegoochoa/calculator-sezzle-api/internal/auth"
	"github.com/diegoochoa/calculator-sezzle-api/internal/calc"
	"github.com/diegoochoa/calculator-sezzle-api/internal/httpx"
)

// TokenIssuer mints bearer tokens.
type TokenIssuer interface {
	Issue(subject string) (auth.Token, error)
}

// ClientAuthenticator checks client credentials.
type ClientAuthenticator interface {
	Authenticate(id, secret string) bool
}

// Server holds the handler dependencies.
type Server struct {
	calcOptions calc.Options
	maxBatch    int
	issuer      TokenIssuer
	clients     ClientAuthenticator
	logger      *slog.Logger
	version     string
	startedAt   time.Time
	now         func() time.Time
}

// ServerConfig configures the handlers.
type ServerConfig struct {
	CalcOptions calc.Options
	MaxBatch    int
	Issuer      TokenIssuer
	Clients     ClientAuthenticator
	Logger      *slog.Logger
	Version     string
}

// NewServer builds the handler set.
func NewServer(cfg ServerConfig) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.MaxBatch <= 0 {
		cfg.MaxBatch = 50
	}
	if cfg.Version == "" {
		cfg.Version = "dev"
	}

	return &Server{
		calcOptions: cfg.CalcOptions,
		maxBatch:    cfg.MaxBatch,
		issuer:      cfg.Issuer,
		clients:     cfg.Clients,
		logger:      cfg.Logger,
		version:     cfg.Version,
		startedAt:   time.Now(),
		now:         time.Now,
	}
}

// engineStatus maps an engine error to a status code. Every engine failure is a
// well-formed request the server understood but cannot compute, which is
// exactly what 422 means; malformed JSON never reaches the engine and is a 400.
func engineStatus(*calc.Error) int {
	return http.StatusUnprocessableEntity
}

// writeEngineError renders an engine failure, carrying the offset so a client
// can underline the offending part of the expression.
func writeEngineError(w http.ResponseWriter, r *http.Request, err *calc.Error) {
	if err.Position == calc.NoPosition {
		httpx.WriteError(w, r, engineStatus(err), err.Code, err.Message)
		return
	}
	httpx.WriteErrorAt(w, r, engineStatus(err), err.Code, err.Message, err.Position)
}

// errorBody converts an engine error into the envelope's error object, for
// endpoints that report failures per item rather than per request.
func errorBody(err *calc.Error) *httpx.ErrorBody {
	body := &httpx.ErrorBody{Code: err.Code, Message: err.Message}
	if err.Position != calc.NoPosition {
		position := err.Position
		body.Position = &position
	}
	return body
}
