package api

import (
	"encoding/json"
	"io"
	"net/http"

	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/services/ingestion/internal/validation"
)

// IngestHandler handles POST /api/v1/telemetry requests.
type IngestHandler struct {
	producer EventProducer
	logger   *zap.Logger
}

// NewIngestHandler creates a new handler with the given producer and logger.
func NewIngestHandler(producer EventProducer, logger *zap.Logger) *IngestHandler {
	return &IngestHandler{
		producer: producer,
		logger:   logger,
	}
}

func (h *IngestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body", "")
		return
	}

	if len(body) == 0 {
		writeError(w, http.StatusBadRequest, "request body must not be empty", "body")
		return
	}

	event, err := validation.Validate(body)
	if err != nil {
		if valErr, ok := err.(*validation.ValidationError); ok {
			writeError(w, http.StatusBadRequest, valErr.Message, valErr.Field)
		} else {
			writeError(w, http.StatusBadRequest, err.Error(), "")
		}
		return
	}

	if err := h.producer.Publish(r.Context(), event); err != nil {
		h.logger.Error("failed to publish event", zap.Error(err))
		writeError(w, http.StatusServiceUnavailable, "failed to publish event", "")
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// errorResponse is the structured error format for the API.
type errorResponse struct {
	Error string `json:"error"`
	Field string `json:"field,omitempty"`
}

func writeError(w http.ResponseWriter, status int, message, field string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorResponse{Error: message, Field: field})
}
