package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/danielkuo/project-plant/services/dashboard/internal/metrics"
)

func TestActiveWebsocketConnections_SamplesClientCount(t *testing.T) {
	count := 0
	m := metrics.New(func() int { return count })

	count = 3
	v, err := testutil.GatherAndCount(m.Gather(), "active_websocket_connections")
	require.NoError(t, err)
	assert.Equal(t, 1, v)

	expected := strings.NewReader(`
# HELP active_websocket_connections WebSocket clients currently connected to the events stream.
# TYPE active_websocket_connections gauge
active_websocket_connections 3
`)
	require.NoError(t, testutil.GatherAndCompare(m.Gather(), expected, "active_websocket_connections"))
}

func TestMetricsHandler_ServesPrometheusFormat(t *testing.T) {
	m := metrics.New(func() int { return 0 })

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "active_websocket_connections")
	assert.Contains(t, rec.Body.String(), "go_memstats_alloc_bytes")
}
