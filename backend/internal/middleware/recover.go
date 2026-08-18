package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/diegoochoa/calculator-sezzle-api/internal/httpx"
)

// Recover turns a panic into a 500 in the standard envelope. It belongs
// outermost so a panic raised in any layer below is still answered rather than
// killing the connection, and the stack reaches the logs instead of the client.
func Recover(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := newRecorder(w)

			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}

				logger.Error("panic recovered",
					"error", recovered,
					"method", r.Method,
					"path", r.URL.Path,
					"requestId", httpx.RequestID(r.Context()),
					"stack", string(debug.Stack()),
				)

				// Once bytes are on the wire the status is fixed; all we can do
				// is stop writing and let the client see a truncated body.
				if rec.wroteHeader {
					return
				}
				httpx.WriteError(rec, r, http.StatusInternalServerError,
					httpx.CodeInternal, "Something went wrong")
			}()

			next.ServeHTTP(rec, r)
		})
	}
}
