package api_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/services/ingestion/internal/api"
	"github.com/danielkuo/project-plant/services/ingestion/internal/metrics"
)

func metricsRouter(t *testing.T) (http.Handler, *metrics.Metrics) {
	t.Helper()
	producer := &mockProducer{}
	noAuth := func(h http.Handler) http.Handler { return h }
	m := metrics.New()
	return api.NewRouter(producer, noAuth, m, zap.NewNop()), m
}

func postTelemetry(t *testing.T, router http.Handler, body []byte) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Code
}

// TestMetricsEndpoint validates GET /metrics serves Prometheus exposition
// format with the service's metric families present.
func TestMetricsEndpoint(t *testing.T) {
	router, _ := metricsRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)

	// Counter family visible at zero (children pre-initialized), histogram
	// declared, and runtime collectors registered.
	assert.Contains(t, string(body), "events_ingested_total")
	assert.Contains(t, string(body), "ingestion_latency_seconds")
	assert.Contains(t, string(body), "go_memstats_alloc_bytes")
}

// TestMetrics_EventsIngested validates the roadmap criterion: after 10 POSTs,
// events_ingested_total{status="accepted"} == 10.
func TestMetrics_EventsIngested(t *testing.T) {
	router, m := metricsRouter(t)

	for i := 0; i < 10; i++ {
		body := []byte(fmt.Sprintf(
			`{"device_id":"dev-%03d","timestamp":"2026-06-09T12:00:00Z","temperature":22.5,"humidity":55,"soil_moisture":40}`, i))
		require.Equal(t, http.StatusAccepted, postTelemetry(t, router, body))
	}

	accepted := testutil.ToFloat64(m.EventsIngestedCounter(metrics.StatusAccepted))
	assert.Equal(t, 10.0, accepted)
	assert.Equal(t, 0.0, testutil.ToFloat64(m.EventsIngestedCounter(metrics.StatusRejected)))
}

// TestMetrics_EventsIngested_BatchAndRejected covers event-level (not
// request-level) counting on the batch route and the rejected path.
func TestMetrics_EventsIngested_BatchAndRejected(t *testing.T) {
	router, m := metricsRouter(t)

	batch := []byte(`[
		{"device_id":"dev-a","timestamp":"2026-06-09T12:00:00Z","temperature":22.5,"humidity":55,"soil_moisture":40},
		{"device_id":"dev-b","timestamp":"2026-06-09T12:00:01Z","temperature":23.5,"humidity":56,"soil_moisture":41},
		{"device_id":"dev-c","timestamp":"2026-06-09T12:00:02Z","temperature":24.5,"humidity":57,"soil_moisture":42}
	]`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry/batch", bytes.NewReader(batch))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusAccepted, rec.Code)

	require.Equal(t, http.StatusBadRequest,
		postTelemetry(t, router, []byte(`{"device_id":"","timestamp":"2026-06-09T12:00:00Z","temperature":22.5,"humidity":55,"soil_moisture":40}`)))

	assert.Equal(t, 3.0, testutil.ToFloat64(m.EventsIngestedCounter(metrics.StatusAccepted)),
		"batch of 3 must count 3 events")
	assert.Equal(t, 1.0, testutil.ToFloat64(m.EventsIngestedCounter(metrics.StatusRejected)))
}
