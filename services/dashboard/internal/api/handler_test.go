package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/pkg/telemetry"
	"github.com/danielkuo/project-plant/services/dashboard/internal/api"
)

// --- Mocks ---

type mockDeviceReader struct{ mock.Mock }

func (m *mockDeviceReader) GetLatest(ctx context.Context, deviceID string) (telemetry.TelemetryEvent, error) {
	args := m.Called(ctx, deviceID)
	return args.Get(0).(telemetry.TelemetryEvent), args.Error(1)
}

func (m *mockDeviceReader) GetAllLatest(ctx context.Context) (map[string]telemetry.TelemetryEvent, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]telemetry.TelemetryEvent), args.Error(1)
}

type mockHistoryReader struct{ mock.Mock }

func (m *mockHistoryReader) QueryByDevice(ctx context.Context, deviceID string, tr api.TimeRange, pg api.Pagination) ([]telemetry.TelemetryEvent, error) {
	args := m.Called(ctx, deviceID, tr, pg)
	return args.Get(0).([]telemetry.TelemetryEvent), args.Error(1)
}

type mockAlertStore struct{ mock.Mock }

func (m *mockAlertStore) QueryAlerts(ctx context.Context, filters api.AlertFilters) ([]api.Alert, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]api.Alert), args.Error(1)
}

func (m *mockAlertStore) ResolveAlert(ctx context.Context, alertID string) error {
	args := m.Called(ctx, alertID)
	return args.Error(0)
}

type mockStatsReader struct{ mock.Mock }

func (m *mockStatsReader) GetStats(ctx context.Context) (api.DeviceStats, error) {
	args := m.Called(ctx)
	return args.Get(0).(api.DeviceStats), args.Error(1)
}

// --- Test helpers ---

func testEvent(deviceID string) telemetry.TelemetryEvent {
	return telemetry.TelemetryEvent{
		DeviceID:     deviceID,
		Timestamp:    time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
		Temperature:  23.5,
		Humidity:     62.3,
		SoilMoisture: 45.1,
	}
}

func newHandler(devices *mockDeviceReader, history *mockHistoryReader, alerts *mockAlertStore, stats *mockStatsReader) *api.Handler {
	return api.NewHandler(devices, history, alerts, stats, zap.NewNop())
}

func newRouter(devices *mockDeviceReader, history *mockHistoryReader, alerts *mockAlertStore, stats *mockStatsReader) http.Handler {
	h := newHandler(devices, history, alerts, stats)
	// Pass nil for wsHandler since REST tests don't need WebSocket
	return api.NewRouter(h, nil, zap.NewNop())
}

// --- REST Handler Tests ---

func TestListDevices(t *testing.T) {
	devices := new(mockDeviceReader)
	devices.On("GetAllLatest", mock.Anything).Return(map[string]telemetry.TelemetryEvent{
		"dev-001": testEvent("dev-001"),
		"dev-002": testEvent("dev-002"),
		"dev-003": testEvent("dev-003"),
	}, nil)

	router := newRouter(devices, new(mockHistoryReader), new(mockAlertStore), new(mockStatsReader))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result []api.DeviceWithStatus
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	assert.Len(t, result, 3)

	// Each device should have a computed status
	for _, d := range result {
		assert.NotEmpty(t, d.DeviceID)
		assert.Contains(t, []string{"normal", "warning", "critical", "stale"}, d.Status)
		assert.NotZero(t, d.Latest.Temperature)
	}
	devices.AssertExpectations(t)
}

func TestListDevices_Empty(t *testing.T) {
	devices := new(mockDeviceReader)
	devices.On("GetAllLatest", mock.Anything).Return(map[string]telemetry.TelemetryEvent{}, nil)

	router := newRouter(devices, new(mockHistoryReader), new(mockAlertStore), new(mockStatsReader))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Must be [] not null
	body := rec.Body.String()
	assert.Contains(t, body, "[]")
}

func TestGetDevice(t *testing.T) {
	devices := new(mockDeviceReader)
	devices.On("GetLatest", mock.Anything, "dev-001").Return(testEvent("dev-001"), nil)

	router := newRouter(devices, new(mockHistoryReader), new(mockAlertStore), new(mockStatsReader))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/dev-001", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result api.DeviceWithStatus
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	assert.Equal(t, "dev-001", result.DeviceID)
	assert.Equal(t, "normal", result.Status)
	devices.AssertExpectations(t)
}

func TestGetDevice_NotFound(t *testing.T) {
	devices := new(mockDeviceReader)
	devices.On("GetLatest", mock.Anything, "unknown").Return(telemetry.TelemetryEvent{}, errors.New("redis: nil"))

	router := newRouter(devices, new(mockHistoryReader), new(mockAlertStore), new(mockStatsReader))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/unknown", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "not found")
}

func TestGetDeviceHistory(t *testing.T) {
	history := new(mockHistoryReader)
	events := []telemetry.TelemetryEvent{testEvent("dev-001"), testEvent("dev-001")}
	history.On("QueryByDevice", mock.Anything, "dev-001", mock.Anything, mock.Anything).Return(events, nil)

	router := newRouter(new(mockDeviceReader), history, new(mockAlertStore), new(mockStatsReader))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/dev-001/history?from=2026-04-10T00:00:00Z&to=2026-04-10T23:59:59Z", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result []telemetry.TelemetryEvent
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	assert.Len(t, result, 2)
	history.AssertExpectations(t)
}

func TestGetDeviceHistory_Pagination(t *testing.T) {
	history := new(mockHistoryReader)
	history.On("QueryByDevice", mock.Anything, "dev-001", mock.Anything, api.Pagination{Limit: 10, Offset: 20}).
		Return([]telemetry.TelemetryEvent{testEvent("dev-001")}, nil)

	router := newRouter(new(mockDeviceReader), history, new(mockAlertStore), new(mockStatsReader))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/dev-001/history?limit=10&offset=20", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	history.AssertExpectations(t)
}

func TestListAlerts(t *testing.T) {
	alerts := new(mockAlertStore)
	now := time.Now().UTC()
	alertList := []api.Alert{
		{AlertID: "a-1", DeviceID: "dev-001", RuleName: "high_temperature", Severity: "warning", TriggeredAt: now},
	}
	alerts.On("QueryAlerts", mock.Anything, api.AlertFilters{}).Return(alertList, nil)

	router := newRouter(new(mockDeviceReader), new(mockHistoryReader), alerts, new(mockStatsReader))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result []api.Alert
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	assert.Len(t, result, 1)
	assert.Equal(t, "warning", result[0].Severity)
	alerts.AssertExpectations(t)
}

func TestListAlerts_FilterBySeverity(t *testing.T) {
	alerts := new(mockAlertStore)
	alerts.On("QueryAlerts", mock.Anything, api.AlertFilters{Severity: "critical"}).
		Return([]api.Alert{}, nil)

	router := newRouter(new(mockDeviceReader), new(mockHistoryReader), alerts, new(mockStatsReader))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts?severity=critical", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	alerts.AssertExpectations(t)
}

func TestListAlerts_FilterByStatus(t *testing.T) {
	alerts := new(mockAlertStore)
	alerts.On("QueryAlerts", mock.Anything, api.AlertFilters{Status: "active"}).
		Return([]api.Alert{}, nil)

	router := newRouter(new(mockDeviceReader), new(mockHistoryReader), alerts, new(mockStatsReader))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts?status=active", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	alerts.AssertExpectations(t)
}

func TestResolveAlert(t *testing.T) {
	alerts := new(mockAlertStore)
	alerts.On("ResolveAlert", mock.Anything, "alert-123").Return(nil)

	router := newRouter(new(mockDeviceReader), new(mockHistoryReader), alerts, new(mockStatsReader))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/alert-123/resolve", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "resolved", resp["status"])
	alerts.AssertExpectations(t)
}

func TestGetStats(t *testing.T) {
	stats := new(mockStatsReader)
	stats.On("GetStats", mock.Anything).Return(api.DeviceStats{
		DeviceCount:  5,
		TotalEvents:  10000,
		ActiveAlerts: 2,
	}, nil)

	router := newRouter(new(mockDeviceReader), new(mockHistoryReader), new(mockAlertStore), stats)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result api.DeviceStats
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	assert.Equal(t, 5, result.DeviceCount)
	assert.Equal(t, int64(10000), result.TotalEvents)
	assert.Equal(t, 2, result.ActiveAlerts)
	stats.AssertExpectations(t)
}

func TestCORS(t *testing.T) {
	router := newRouter(new(mockDeviceReader), new(mockHistoryReader), new(mockAlertStore), new(mockStatsReader))

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/devices", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "GET")
}
