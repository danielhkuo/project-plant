package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/pkg/telemetry"
	"github.com/danielkuo/project-plant/services/ingestion/internal/validation"
)

// BatchIngestHandler handles POST /api/v1/telemetry/batch requests, accepting a
// JSON array of telemetry events. The batch is validated atomically: if any
// element is invalid the whole batch is rejected (400) and nothing is
// published; otherwise every event is published and the handler returns 202.
type BatchIngestHandler struct {
	producer EventProducer
	logger   *zap.Logger
}

// NewBatchIngestHandler creates a batch handler with the given producer and logger.
func NewBatchIngestHandler(producer EventProducer, logger *zap.Logger) *BatchIngestHandler {
	return &BatchIngestHandler{producer: producer, logger: logger}
}

func (h *BatchIngestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body", "")
		return
	}
	if len(body) == 0 {
		writeError(w, http.StatusBadRequest, "request body must not be empty", "body")
		return
	}

	var rawEvents []json.RawMessage
	if err := json.Unmarshal(body, &rawEvents); err != nil {
		writeError(w, http.StatusBadRequest, "request body must be a JSON array of events", "")
		return
	}
	if len(rawEvents) == 0 {
		writeError(w, http.StatusBadRequest, "batch must contain at least one event", "")
		return
	}

	// Validate all events first so the batch is all-or-nothing.
	events := make([]telemetry.TelemetryEvent, 0, len(rawEvents))
	for i, raw := range rawEvents {
		event, err := validation.Validate(raw)
		if err != nil {
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
			h.logger.Error("failed to publish batch event", zap.Int("index", i), zap.Error(err))
			writeError(w, http.StatusServiceUnavailable, "failed to publish event", "")
			return
		}
	}

	w.WriteHeader(http.StatusAccepted)
}
