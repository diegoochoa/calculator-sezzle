package httpapi

import (
	"net/http"
	"strings"

	"github.com/diegoochoa/calculator-sezzle-api/internal/httpx"
	"github.com/diegoochoa/calculator-sezzle-api/internal/middleware"
)

// RouterConfig wires the handlers to the middleware stack.
type RouterConfig struct {
	Server   *Server
	Verifier middleware.Verifier

	// APILimiter bounds authenticated calculation traffic per client.
	APILimiter *middleware.Limiter
	// AuthLimiter bounds token requests per source address. It is separate and
	// stricter: bcrypt makes that route the expensive one and the obvious
	// brute-force target.
	AuthLimiter *middleware.Limiter

	IPResolver   *middleware.IPResolver
	CORSOrigins  []string
	MaxBodyBytes int64
}

// NewRouter builds the full handler: global layers, then per-route layers.
//
// Order is deliberate. Recovery is outermost so a panic anywhere below still
// produces the JSON envelope. Authentication precedes rate limiting so the
// limiter can key on the authenticated client rather than a shared NAT address.
func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()
	server := cfg.Server

	protected := func(handler http.Handler) http.Handler {
		return middleware.Chain(handler,
			middleware.Authenticate(cfg.Verifier),
			middleware.RateLimit(cfg.APILimiter, middleware.SubjectKey(cfg.IPResolver)),
		)
	}
	throttledByIP := func(handler http.Handler) http.Handler {
		return middleware.Chain(handler,
			middleware.RateLimit(cfg.AuthLimiter, middleware.IPKey(cfg.IPResolver)),
		)
	}

	route(mux, http.MethodPost, "/v1/auth/token", http.HandlerFunc(server.handleToken), throttledByIP)
	route(mux, http.MethodPost, "/v1/calculate", http.HandlerFunc(server.handleCalculate), protected)
	route(mux, http.MethodPost, "/v1/calculate/batch", http.HandlerFunc(server.handleBatch), protected)
	route(mux, http.MethodPost, "/v1/validate", http.HandlerFunc(server.handleValidate), protected)
	route(mux, http.MethodGet, "/v1/functions", http.HandlerFunc(server.handleFunctions), protected)

	// Probes stay unauthenticated so an orchestrator does not need credentials.
	route(mux, http.MethodGet, "/healthz", http.HandlerFunc(server.handleHealth), nil)
	route(mux, http.MethodGet, "/readyz", http.HandlerFunc(server.handleReady), nil)

	// Documentation is unauthenticated too: a client has to read it to learn how
	// to authenticate in the first place.
	route(mux, http.MethodGet, SpecPath, http.HandlerFunc(server.handleOpenAPISpec), nil)
	route(mux, http.MethodGet, DocsPath, http.HandlerFunc(server.handleDocs), nil)

	mux.Handle("/", http.HandlerFunc(notFound))

	return middleware.Chain(mux,
		middleware.Recover(server.logger),
		middleware.RequestID,
		middleware.Logging(server.logger),
		middleware.CORS(cfg.CORSOrigins),
		middleware.BodyLimit(cfg.MaxBodyBytes),
	)
}

// route registers a method-specific handler plus a bare-path fallback, so a
// wrong method returns the JSON envelope instead of ServeMux's plain text.
func route(mux *http.ServeMux, method, path string, handler http.Handler, wrap func(http.Handler) http.Handler) {
	if wrap != nil {
		handler = wrap(handler)
	}
	mux.Handle(method+" "+path, handler)
	mux.Handle(path, methodNotAllowed(method))
}

func methodNotAllowed(allowed ...string) http.Handler {
	allow := strings.Join(append(allowed, http.MethodOptions), ", ")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", allow)
		httpx.WriteError(w, r, http.StatusMethodNotAllowed, httpx.CodeMethodNotAllowed,
			"This endpoint accepts "+allow)
	})
}

func notFound(w http.ResponseWriter, r *http.Request) {
	httpx.WriteError(w, r, http.StatusNotFound, httpx.CodeNotFound, "No such endpoint")
}
