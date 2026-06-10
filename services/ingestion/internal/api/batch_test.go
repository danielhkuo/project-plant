package api_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/services/ingestion/internal/api"
)

func validBatch(n int) []byte {
	out := []byte("[")
	for i := 0; i < n; i++ {
		if i > 0 {
			out = append(out, ',')
		}
		out = append(out, validJSON()...)
	}
	return append(out, ']')
}

func TestBatchHandler_ValidArray(t *testing.T) {
	producer := &mockProducer{}
	handler := api.NewBatchIngestHandler(producer, nil, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry/batch", bytes.NewReader(validBatch(3)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.Equal(t, 3, producer.publishCalled)
}

func TestBatchHandler_OneInvalidRejectsWholeBatch(t *testing.T) {
	producer := &mockProducer{}
	handler := api.NewBatchIngestHandler(producer, nil, zap.NewNop())

	// Second element has an empty device_id.
	body := []byte(`[` + string(validJSON()) + `,` +
		`{"device_id":"","timestamp":"2026-04-09T12:00:00Z","temperature":23.5,"humidity":62.3,"soil_moisture":45.1}]`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, 0, producer.publishCalled, "no events should publish when any element is invalid")
}

func TestBatchHandler_NotAnArray(t *testing.T) {
	producer := &mockProducer{}
	handler := api.NewBatchIngestHandler(producer, nil, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry/batch", bytes.NewReader(validJSON()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, 0, producer.publishCalled)
}

func TestBatchHandler_EmptyArray(t *testing.T) {
	producer := &mockProducer{}
	handler := api.NewBatchIngestHandler(producer, nil, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry/batch", bytes.NewReader([]byte(`[]`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, 0, producer.publishCalled)
}
