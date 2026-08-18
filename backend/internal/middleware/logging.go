package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/diegoochoa/calculator-sezzle-api/internal/httpx"
)

// Logging emits one structured line per request. Server errors are logged at
// error level so they surface in alerting without parsing status codes out of
// info lines.
func Logging(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			rec := newRecorder(w)

			next.ServeHTTP(rec, r)

			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"bytes", rec.bytes,
				"duration", time.Since(started).String(),
				"requestId", httpx.RequestID(r.Context()),
			}
			// Only present once authentication has run.
			if subject := httpx.Subject(r.Context()); subject != "" {
				attrs = append(attrs, "client", subject)
			}

			switch {
			case rec.status >= http.StatusInternalServerError:
				logger.Error("request failed", attrs...)
			case rec.status >= http.StatusBadRequest:
				logger.Warn("request rejected", attrs...)
			default:
				logger.Info("request served", attrs...)
			}
		})
	}
}
