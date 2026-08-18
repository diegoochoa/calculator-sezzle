package httpapi

import (
	"net/http"
	"time"

	"github.com/diegoochoa/calculator-sezzle-api/internal/httpx"
)

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Uptime  string `json:"uptime,omitempty"`
}

// handleHealth answers liveness probes: the process is up and serving.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, healthResponse{Status: "ok", Version: s.version})
}

// handleReady answers readiness probes. The engine has no external
// dependencies, so readiness is liveness plus uptime; when a datastore or cache
// appears, its check belongs here rather than in handleHealth, so a failing
// dependency stops traffic without restarting the pod.
func (s *Server) handleReady(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, healthResponse{
		Status:  "ready",
		Version: s.version,
		Uptime:  s.now().Sub(s.startedAt).Truncate(time.Second).String(),
	})
}
