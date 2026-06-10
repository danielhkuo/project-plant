package api

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/pkg/telemetry"
)

// Handler serves dashboard REST endpoints.
type Handler struct {
	devices DeviceReader
	history HistoryReader
	alerts  AlertStore
	stats   StatsReader
	logger  *zap.Logger
}

// NewHandler creates a handler with all required dependencies.
func NewHandler(devices DeviceReader, history HistoryReader, alerts AlertStore, stats StatsReader, logger *zap.Logger) *Handler {
	return &Handler{
		devices: devices,
		history: history,
		alerts:  alerts,
		stats:   stats,
		logger:  logger,
	}
}

// ListDevices handles GET /api/v1/devices.
func (h *Handler) ListDevices(w http.ResponseWriter, r *http.Request) {
	latest, err := h.devices.GetAllLatest(r.Context())
	if err != nil {
		h.logger.Error("failed to get devices", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to retrieve devices", "")
		return
	}

	result := make([]DeviceWithStatus, 0, len(latest))
	for id, event := range latest {
		result = append(result, DeviceWithStatus{
			DeviceID: id,
			Status:   computeStatus(event),
			Latest:   event,
		})
	}

	// Sort by device ID for stable ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].DeviceID < result[j].DeviceID
	})

	writeJSON(w, http.StatusOK, result)
}

// GetDevice handles GET /api/v1/devices/{id}.
func (h *Handler) GetDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	event, err := h.devices.GetLatest(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "device not found", "device_id")
		return
	}

	writeJSON(w, http.StatusOK, DeviceWithStatus{
		DeviceID: id,
		Status:   computeStatus(event),
		Latest:   event,
	})
}

// GetDeviceHistory handles GET /api/v1/devices/{id}/history.
func (h *Handler) GetDeviceHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	q := r.URL.Query()

	tr := TimeRange{
		From: parseTime(q.Get("from"), time.Now().Add(-1*time.Hour)),
		To:   parseTime(q.Get("to"), time.Now()),
	}
	pg := Pagination{
		Limit:  parseInt(q.Get("limit"), 100),
		Offset: parseInt(q.Get("offset"), 0),
	}

	events, err := h.history.QueryByDevice(r.Context(), id, tr, pg)
	if err != nil {
		h.logger.Error("failed to query history", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to retrieve history", "")
		return
	}

	if events == nil {
		events = []telemetry.TelemetryEvent{}
	}
	writeJSON(w, http.StatusOK, events)
}

// ListAlerts handles GET /api/v1/alerts.
func (h *Handler) ListAlerts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filters := AlertFilters{
		DeviceID: q.Get("device_id"),
		Severity: q.Get("severity"),
		Status:   q.Get("status"),
	}

	alerts, err := h.alerts.QueryAlerts(r.Context(), filters)
	if err != nil {
		h.logger.Error("failed to query alerts", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to retrieve alerts", "")
		return
	}

	if alerts == nil {
		alerts = []Alert{}
	}
	writeJSON(w, http.StatusOK, alerts)
}

// ResolveAlert handles POST /api/v1/alerts/{id}/resolve.
func (h *Handler) ResolveAlert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.alerts.ResolveAlert(r.Context(), id); err != nil {
		h.logger.Error("failed to resolve alert", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to resolve alert", "")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"alert_id": id, "status": "resolved"})
}

// GetStats handles GET /api/v1/stats.
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.stats.GetStats(r.Context())
	if err != nil {
		h.logger.Error("failed to get stats", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to retrieve stats", "")
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// computeStatus determines device status from latest reading.
// Maps to Nothing design system status tokens:
//
//	normal   -> --success (#4A9E5C)
//	warning  -> --warning (#D4A843)
//	critical -> --accent  (#D71921)
//	stale    -> --text-disabled (#666666)
func computeStatus(event telemetry.TelemetryEvent) string {
	// Critical thresholds (from rule engine defaults)
	if event.Temperature > 60 || event.SoilMoisture < 10 {
		return "critical"
	}
	// Warning thresholds
	if event.Temperature > 40 || event.Temperature < 5 ||
		event.SoilMoisture < 20 || event.SoilMoisture > 90 {
		return "warning"
	}
	return "normal"
}

func parseTime(s string, fallback time.Time) time.Time {
	if s == "" {
		return fallback
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return fallback
	}
	return t
}

func parseInt(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return v
}

// ParseDeviceFilter parses a comma-separated device filter string.
func ParseDeviceFilter(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
