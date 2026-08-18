package middleware

import "net/http"

// BodyLimit caps how much of a request body a handler can read. Without it a
// single client can stream an unbounded body into memory before any validation
// runs. Handlers turn the resulting *http.MaxBytesError into a 413.
func BodyLimit(maxBytes int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}
