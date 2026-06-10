package health_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/danielkuo/project-plant/pkg/health"
)

func get(t *testing.T, checks map[string]health.Check) (int, map[string]string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	health.Handler(checks)(rec, req)

	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return rec.Code, body
}

func ok(context.Context) error   { return nil }
func fail(context.Context) error { return errors.New("dependency down") }

func TestHealth_AllConnected(t *testing.T) {
	code, body := get(t, map[string]health.Check{"kafka": ok, "postgres": ok, "redis": ok})

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ok", body["status"])
	assert.Equal(t, "connected", body["kafka"])
	assert.Equal(t, "connected", body["postgres"])
	assert.Equal(t, "connected", body["redis"])
}

func TestHealth_OneFailing_Degraded(t *testing.T) {
	code, body := get(t, map[string]health.Check{"kafka": ok, "postgres": fail})

	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, "degraded", body["status"])
	assert.Equal(t, "connected", body["kafka"])
	assert.Equal(t, "disconnected", body["postgres"])
}

func TestHealth_SingleDependency_MatchesIngestionShape(t *testing.T) {
	// The ingestion integration tests assert this exact JSON shape.
	code, body := get(t, map[string]health.Check{"kafka": ok})
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, map[string]string{"status": "ok", "kafka": "connected"}, body)

	code, body = get(t, map[string]health.Check{"kafka": fail})
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, map[string]string{"status": "degraded", "kafka": "disconnected"}, body)
}

func TestHealth_SlowCheck_TimesOutAsDisconnected(t *testing.T) {
	hung := func(ctx context.Context) error {
		<-ctx.Done() // never answers on its own; respects cancellation
		return ctx.Err()
	}

	start := time.Now()
	code, body := get(t, map[string]health.Check{"kafka": ok, "redis": hung})

	assert.Less(t, time.Since(start), 4*time.Second, "handler must enforce its deadline")
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, "degraded", body["status"])
	assert.Equal(t, "disconnected", body["redis"])
}
