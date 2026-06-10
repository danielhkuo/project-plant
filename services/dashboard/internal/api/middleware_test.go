package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/danielkuo/project-plant/services/dashboard/internal/api"
)

func TestRequestIDMiddleware(t *testing.T) {
	t.Run("generates ID when none provided", func(t *testing.T) {
		handler := api.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.NotEmpty(t, rec.Header().Get("X-Request-ID"))
	})

	t.Run("preserves client-provided ID", func(t *testing.T) {
		handler := api.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Request-ID", "client-123")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, "client-123", rec.Header().Get("X-Request-ID"))
	})
}

// TestRequestLogFields validates the request-log contract: every completed
// request logs method, path, status, duration_ms, and request_id. The chain
// mirrors the router (RequestID wraps Logger) so the id is in context.
func TestRequestLogFields(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	handler := api.RequestID(api.Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/test-path", nil)
	req.Header.Set("X-Request-ID", "req-dash-456")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, 1, logs.Len())

	entry := logs.All()[0]
	assert.Equal(t, "request completed", entry.Message)

	fields := entry.ContextMap()
	assert.Equal(t, "GET", fields["method"])
	assert.Equal(t, "/test-path", fields["path"])
	assert.Equal(t, int64(http.StatusOK), fields["status"])
	assert.Contains(t, fields, "duration_ms")
	assert.IsType(t, float64(0), fields["duration_ms"])
	assert.Equal(t, "req-dash-456", fields["request_id"])
}

func TestRecoveryMiddleware(t *testing.T) {
	logger := zap.NewNop()

	handler := api.Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("unexpected error")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestCORSMiddleware(t *testing.T) {
	handler := api.CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("OPTIONS returns CORS headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/api/v1/devices", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "POST")
	})

	t.Run("regular request includes origin header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
	})
}
