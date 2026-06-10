package consumer

import (
	"context"

	"github.com/danielkuo/project-plant/pkg/telemetry"
	"github.com/danielkuo/project-plant/services/processor/internal/engine"
)

// EventStore abstracts the persistence layer for telemetry events.
type EventStore interface {
	Write(ctx context.Context, event telemetry.TelemetryEvent) error
	SetLatest(ctx context.Context, deviceID string, event telemetry.TelemetryEvent) error
	WriteAlert(ctx context.Context, alert engine.Alert) error
}

// AlertPublisher abstracts the real-time alert delivery mechanism.
type AlertPublisher interface {
	Publish(ctx context.Context, alert engine.Alert) error
}

// Metrics observes processing outcomes. It keeps the metrics backend
// (Prometheus) out of the processing logic; implementations must be safe for
// concurrent use.
type Metrics interface {
	// EventProcessed records one message outcome: "success" or "error".
	EventProcessed(result string)
	// AlertFired records one triggered alert by rule name and severity.
	AlertFired(rule, severity string)
}
