package api

import (
	"net/http"

	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/pkg/health"
	"github.com/danielkuo/project-plant/services/dashboard/internal/metrics"
)

// NewRouter creates and configures the HTTP router with all routes and middleware.
//
// checks are the dependency probes served by /health (postgres, redis). m may
// be nil, which disables the /metrics route (used by handler-only tests).
func NewRouter(h *Handler, wsHandler http.Handler, m *metrics.Metrics, checks map[string]health.Check, logger *zap.Logger) http.Handler {
	mux := http.NewServeMux()

	// REST endpoints
	mux.HandleFunc("GET /api/v1/devices", h.ListDevices)
	mux.HandleFunc("GET /api/v1/devices/{id}", h.GetDevice)
	mux.HandleFunc("GET /api/v1/devices/{id}/history", h.GetDeviceHistory)
	mux.HandleFunc("GET /api/v1/alerts", h.ListAlerts)
	mux.HandleFunc("POST /api/v1/alerts/{id}/resolve", h.ResolveAlert)
	mux.HandleFunc("GET /api/v1/stats", h.GetStats)

	// Dependency-aware health probe + Prometheus scrape endpoint.
	mux.Handle("GET /health", health.Handler(checks))
	if m != nil {
		mux.Handle("GET /metrics", m.Handler())
	}

	// WebSocket (may be nil during tests)
	if wsHandler != nil {
		mux.Handle("GET /api/v1/ws/events", wsHandler)
	}

	// Middleware chain: CORS -> Recovery -> RequestID -> Logger -> routes.
	// RequestID must wrap Logger so the request id is in context when the
	// completion log line is written.
	var handler http.Handler = mux
	handler = Logger(logger)(handler)
	handler = RequestID(handler)
	handler = Recovery(logger)(handler)
	handler = CORS(handler)

	return handler
}
