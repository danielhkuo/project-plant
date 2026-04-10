package api

import (
	"net/http"

	"go.uber.org/zap"
)

// NewRouter creates and configures the HTTP router with all routes and middleware.
func NewRouter(h *Handler, wsHandler http.Handler, logger *zap.Logger) http.Handler {
	mux := http.NewServeMux()

	// REST endpoints
	mux.HandleFunc("GET /api/v1/devices", h.ListDevices)
	mux.HandleFunc("GET /api/v1/devices/{id}", h.GetDevice)
	mux.HandleFunc("GET /api/v1/devices/{id}/history", h.GetDeviceHistory)
	mux.HandleFunc("GET /api/v1/alerts", h.ListAlerts)
	mux.HandleFunc("POST /api/v1/alerts/{id}/resolve", h.ResolveAlert)
	mux.HandleFunc("GET /api/v1/stats", h.GetStats)
	mux.HandleFunc("GET /health", h.Health)

	// WebSocket (may be nil during tests)
	if wsHandler != nil {
		mux.Handle("GET /api/v1/ws/events", wsHandler)
	}

	// Apply middleware chain: CORS -> Recovery -> Logger -> RequestID -> routes
	var handler http.Handler = mux
	handler = RequestID(handler)
	handler = Logger(logger)(handler)
	handler = Recovery(logger)(handler)
	handler = CORS(handler)

	return handler
}
