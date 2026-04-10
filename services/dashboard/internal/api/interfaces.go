package api

import (
	"context"
	"time"

	"github.com/danielkuo/project-plant/pkg/telemetry"
)

// TimeRange specifies a time window for historical queries.
type TimeRange struct {
	From time.Time
	To   time.Time
}

// Pagination specifies limit and offset for paginated queries.
type Pagination struct {
	Limit  int
	Offset int
}

// AlertFilters defines query filters for alerts.
type AlertFilters struct {
	DeviceID string
	Severity string
	Status   string // "active" or "resolved"
}

// Alert represents a triggered alerting rule with resolution state.
type Alert struct {
	AlertID     string                   `json:"alert_id"`
	DeviceID    string                   `json:"device_id"`
	RuleName    string                   `json:"rule_name"`
	Severity    string                   `json:"severity"`
	TriggeredAt time.Time                `json:"triggered_at"`
	ResolvedAt  *time.Time               `json:"resolved_at,omitempty"`
	Reading     telemetry.TelemetryEvent `json:"reading"`
}

// DeviceStats holds aggregate fleet statistics.
type DeviceStats struct {
	DeviceCount  int   `json:"device_count"`
	TotalEvents  int64 `json:"total_events"`
	ActiveAlerts int   `json:"active_alerts"`
}

// DeviceWithStatus wraps a device's latest reading with a computed status
// for direct mapping to Nothing design system status colors.
type DeviceWithStatus struct {
	DeviceID string                   `json:"device_id"`
	Status   string                   `json:"status"` // normal, warning, critical, stale
	Latest   telemetry.TelemetryEvent `json:"latest"`
}

// DeviceReader provides access to current device state from the hot cache.
type DeviceReader interface {
	GetLatest(ctx context.Context, deviceID string) (telemetry.TelemetryEvent, error)
	GetAllLatest(ctx context.Context) (map[string]telemetry.TelemetryEvent, error)
}

// HistoryReader provides access to historical telemetry data.
type HistoryReader interface {
	QueryByDevice(ctx context.Context, deviceID string, tr TimeRange, pg Pagination) ([]telemetry.TelemetryEvent, error)
}

// AlertStore provides access to alerts including resolution.
type AlertStore interface {
	QueryAlerts(ctx context.Context, filters AlertFilters) ([]Alert, error)
	ResolveAlert(ctx context.Context, alertID string) error
}

// StatsReader provides aggregate fleet statistics.
type StatsReader interface {
	GetStats(ctx context.Context) (DeviceStats, error)
}

// HealthChecker checks the status of a dependency.
type HealthChecker interface {
	Ping(ctx context.Context) error
}
