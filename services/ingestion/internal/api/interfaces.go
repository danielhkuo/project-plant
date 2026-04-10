package api

import (
	"context"

	"github.com/danielkuo/project-plant/pkg/telemetry"
)

// EventProducer abstracts the downstream message queue.
// In production this is Kafka; in tests it's a mock.
type EventProducer interface {
	Publish(ctx context.Context, event telemetry.TelemetryEvent) error
	Close() error
}
