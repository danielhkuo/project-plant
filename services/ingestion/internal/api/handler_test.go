package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/pkg/telemetry"
	"github.com/danielkuo/project-plant/services/ingestion/internal/api"
)

// mockProducer implements api.EventProducer for testing.
type mockProducer struct {
	publishCalled int
	lastEvent     telemetry.TelemetryEvent
	err           error
}

func (m *mockProducer) Publish(_ context.Context, event telemetry.TelemetryEvent) error {
	m.publishCalled++
	m.lastEvent = event
	return m.err
}

func (m *mockProducer) Close() error { return nil }

func validJSON() []byte {
	return []byte(`{"device_id":"dev-001","timestamp":"2026-04-09T12:00:00Z","temperature":23.5,"humidity":62.3,"soil_moisture":45.1}`)
}

func TestIngestHandler_ValidPayload(t *testing.T) {
	producer := &mockProducer{}
	handler := api.NewIngestHandler(producer, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry", bytes.NewReader(validJSON()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.Equal(t, 1, producer.publishCalled)
	assert.Equal(t, "dev-001", producer.lastEvent.DeviceID)
}

func TestIngestHandler_InvalidPayload(t *testing.T) {
	producer := &mockProducer{}
	handler := api.NewIngestHandler(producer, zap.NewNop())

	body := []byte(`{"device_id":"","timestamp":"2026-04-09T12:00:00Z","temperature":23.5,"humidity":62.3,"soil_moisture":45.1}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, 0, producer.publishCalled)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "device_id")
}

func TestIngestHandler_MethodNotAllowed(t *testing.T) {
	producer := &mockProducer{}
	logger := zap.NewNop()
	router := api.NewRouter(producer, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/telemetry", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestIngestHandler_EmptyBody(t *testing.T) {
	producer := &mockProducer{}
	handler := api.NewIngestHandler(producer, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry", bytes.NewReader([]byte{}))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, 0, producer.publishCalled)
}

func TestIngestHandler_ProducerError(t *testing.T) {
	producer := &mockProducer{err: errors.New("kafka unavailable")}
	handler := api.NewIngestHandler(producer, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry", bytes.NewReader(validJSON()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestIngestHandler_ContentType(t *testing.T) {
	producer := &mockProducer{}
	logger := zap.NewNop()
	router := api.NewRouter(producer, logger)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry", bytes.NewReader(validJSON()))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnsupportedMediaType, rec.Code)
}
