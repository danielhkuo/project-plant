package api

import (
	"net/http"

	"go.uber.org/zap"
)

// NewRouter creates and configures the HTTP router with all routes and middleware.
//
// authMiddleware is applied per-route to the /api/v1/* ingestion endpoints only;
// /health is left public so monitoring can probe it, and CORS preflight
// (OPTIONS) is handled by the CORS middleware without requiring an API key.
func NewRouter(producer EventProducer, authMiddleware func(http.Handler) http.Handler, logger *zap.Logger) http.Handler {
	mux := http.NewServeMux()

	ingestHandler := NewIngestHandler(producer, logger)
	mux.Handle("POST /api/v1/telemetry", authMiddleware(ContentType(ingestHandler)))

	batchHandler := NewBatchIngestHandler(producer, logger)
	mux.Handle("POST /api/v1/telemetry/batch", authMiddleware(ContentType(batchHandler)))

	// Public, unauthenticated health probe with Kafka connectivity status.
	mux.Handle("GET /health", HealthHandler(producer))

	// Apply middleware chain: CORS -> Recovery -> Logger -> RequestID -> routes
	var handler http.Handler = mux
	handler = RequestID(handler)
	handler = Logger(logger)(handler)
	handler = Recovery(logger)(handler)
	handler = CORS(handler)

	return handler
}
