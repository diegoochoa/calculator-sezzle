package middleware

import (
	"net/http"
	"strings"
)

const corsMaxAge = "600"

// CORS answers preflights and tags responses for the configured origins.
//
// Credentials are deliberately not allowed: the API authenticates with a bearer
// header rather than a cookie, so there is no reason to widen the browser's
// trust model. The allowlist is echoed rather than reflected blindly, and Vary
// is always set so a cache never serves one origin's response to another.
func CORS(allowedOrigins []string) Middleware {
	allowAny := false
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if origin == "*" {
			allowAny = true
			continue
		}
		allowed[strings.ToLower(origin)] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			w.Header().Add("Vary", "Origin")

			if origin != "" && (allowAny || allowed[strings.ToLower(origin)]) {
				if allowAny {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
			}

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.Header().Add("Vary", "Access-Control-Request-Method")
				w.Header().Add("Vary", "Access-Control-Request-Headers")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, "+HeaderRequestID)
				w.Header().Set("Access-Control-Max-Age", corsMaxAge)
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
