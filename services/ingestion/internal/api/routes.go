package api

import (
	"net/http"

	"go.uber.org/zap"
)

// NewRouter creates and configures the HTTP router with all routes and middleware.
func NewRouter(producer EventProducer, logger *zap.Logger) http.Handler {
	mux := http.NewServeMux()

	ingestHandler := NewIngestHandler(producer, logger)
	mux.Handle("POST /api/v1/telemetry", ContentType(ingestHandler))

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Apply middleware chain: CORS -> Recovery -> Logger -> RequestID -> routes
	var handler http.Handler = mux
	handler = RequestID(handler)
	handler = Logger(logger)(handler)
	handler = Recovery(logger)(handler)
	handler = CORS(handler)

	return handler
}
