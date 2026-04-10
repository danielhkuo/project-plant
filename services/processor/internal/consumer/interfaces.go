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
