// Package middleware holds the cross-cutting HTTP layers: request identity,
// panic recovery, logging, CORS, body limits, authentication and rate limiting.
package middleware

import "net/http"

// Middleware wraps a handler.
type Middleware func(http.Handler) http.Handler

// Chain applies middleware so that the first argument is the outermost layer.
// Order matters: recovery must be outermost to catch panics raised by any layer
// below it, and authentication must precede rate limiting so the limiter can
// key on the authenticated client.
func Chain(handler http.Handler, layers ...Middleware) http.Handler {
	for i := len(layers) - 1; i >= 0; i-- {
		handler = layers[i](handler)
	}
	return handler
}

// recorder tracks what was actually sent, so logging can report the status and
// recovery can tell whether it is still safe to write an error body.
type recorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func newRecorder(w http.ResponseWriter) *recorder {
	return &recorder{ResponseWriter: w, status: http.StatusOK}
}

func (r *recorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.status = status
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(status)
}

func (r *recorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}
