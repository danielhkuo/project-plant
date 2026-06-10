package api

import (
	"net/http"

	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/pkg/health"
	"github.com/danielkuo/project-plant/services/ingestion/internal/metrics"
)

// NewRouter creates and configures the HTTP router with all routes and middleware.
//
// authMiddleware is applied per-route to the /api/v1/* ingestion endpoints only;
// /health and /metrics are left public so monitoring can probe and scrape them,
// and CORS preflight (OPTIONS) is handled by the CORS middleware without
// requiring an API key. m may be nil, which disables instrumentation and the
// /metrics route.
func NewRouter(producer EventProducer, authMiddleware func(http.Handler) http.Handler, m *metrics.Metrics, logger *zap.Logger) http.Handler {
	mux := http.NewServeMux()

	// InstrumentIngest wraps outside auth so the latency histogram measures
	// what clients experience, including 401s.
	ingestHandler := NewIngestHandler(producer, m, logger)
	mux.Handle("POST /api/v1/telemetry", m.InstrumentIngest(authMiddleware(ContentType(ingestHandler))))

	batchHandler := NewBatchIngestHandler(producer, m, logger)
	mux.Handle("POST /api/v1/telemetry/batch", m.InstrumentIngest(authMiddleware(ContentType(batchHandler))))

	// Public, unauthenticated health probe with Kafka connectivity status.
	mux.Handle("GET /health", health.Handler(map[string]health.Check{
		"kafka": producer.Healthy,
	}))

	// Public Prometheus scrape endpoint.
	if m != nil {
		mux.Handle("GET /metrics", m.Handler())
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
