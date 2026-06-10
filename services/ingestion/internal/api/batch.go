package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/pkg/telemetry"
	"github.com/danielkuo/project-plant/services/ingestion/internal/metrics"
	"github.com/danielkuo/project-plant/services/ingestion/internal/validation"
)

// BatchIngestHandler handles POST /api/v1/telemetry/batch requests, accepting a
// JSON array of telemetry events. The batch is validated atomically: if any
// element is invalid the whole batch is rejected (400) and nothing is
// published; otherwise every event is published and the handler returns 202.
type BatchIngestHandler struct {
	producer EventProducer
	metrics  *metrics.Metrics
	logger   *zap.Logger
}

// NewBatchIngestHandler creates a batch handler with the given producer and
// logger. m may be nil, which disables metric collection.
func NewBatchIngestHandler(producer EventProducer, m *metrics.Metrics, logger *zap.Logger) *BatchIngestHandler {
	return &BatchIngestHandler{producer: producer, metrics: m, logger: logger}
}

func (h *BatchIngestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.metrics.EventsIngested(metrics.StatusRejected, 1)
		writeError(w, http.StatusBadRequest, "failed to read request body", "")
		return
	}
	if len(body) == 0 {
		h.metrics.EventsIngested(metrics.StatusRejected, 1)
		writeError(w, http.StatusBadRequest, "request body must not be empty", "body")
		return
	}

	var rawEvents []json.RawMessage
	if err := json.Unmarshal(body, &rawEvents); err != nil {
		h.metrics.EventsIngested(metrics.StatusRejected, 1)
		writeError(w, http.StatusBadRequest, "request body must be a JSON array of events", "")
		return
	}
	if len(rawEvents) == 0 {
		h.metrics.EventsIngested(metrics.StatusRejected, 1)
		writeError(w, http.StatusBadRequest, "batch must contain at least one event", "")
		return
	}

	// Validate all events first so the batch is all-or-nothing.
	events := make([]telemetry.TelemetryEvent, 0, len(rawEvents))
	for i, raw := range rawEvents {
		event, err := validation.Validate(raw)
		if err != nil {
			// The whole batch is rejected, so every event in it counts.
			h.metrics.EventsIngested(metrics.StatusRejected, len(rawEvents))
			field := ""
			if valErr, ok := err.(*validation.ValidationError); ok {
				field = valErr.Field
			}
			writeError(w, http.StatusBadRequest,
				fmt.Sprintf("event %d: %s", i, err.Error()), field)
			return
		}
		events = append(events, event)
	}

	for i, event := range events {
		if err := h.producer.Publish(r.Context(), event); err != nil {
			h.metrics.EventsIngested(metrics.StatusError, len(events))
			// A canceled request context means the client disconnected (or
			// the server is draining) mid-publish — normal, not a fault.
			if r.Context().Err() != nil {
				h.logger.Warn("client canceled request during batch publish",
					zap.Int("index", i), zap.Error(err))
				return
			}
			h.logger.Error("failed to publish batch event", zap.Int("index", i), zap.Error(err))
			writeError(w, http.StatusServiceUnavailable, "failed to publish event", "")
			return
		}
	}

	h.metrics.EventsIngested(metrics.StatusAccepted, len(events))
	w.WriteHeader(http.StatusAccepted)
}
