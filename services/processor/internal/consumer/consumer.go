package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/pkg/telemetry"
	"github.com/danielkuo/project-plant/services/processor/internal/engine"
)

const maxRetries = 3

// Consumer processes telemetry messages from Kafka.
type Consumer struct {
	engine   *engine.RuleEngine
	store    EventStore
	alertPub AlertPublisher
	logger   *zap.Logger
}

// NewConsumer creates a new Consumer with the given dependencies.
func NewConsumer(eng *engine.RuleEngine, store EventStore, alertPub AlertPublisher, logger *zap.Logger) *Consumer {
	return &Consumer{
		engine:   eng,
		store:    store,
		alertPub: alertPub,
		logger:   logger,
	}
}

// Process handles a single raw message from Kafka.
// Returns nil if the message was processed (or skipped if malformed).
// Returns an error only if a retryable store/publish failure persists.
func (c *Consumer) Process(ctx context.Context, msg []byte) error {
	var event telemetry.TelemetryEvent
	if err := json.Unmarshal(msg, &event); err != nil {
		c.logger.Error("skipping malformed message", zap.Error(err))
		return nil // skip malformed, don't retry
	}

	// Write to durable storage with retries
	if err := c.withRetry(ctx, "store.Write", func() error {
		return c.store.Write(ctx, event)
	}); err != nil {
		return fmt.Errorf("store.Write failed after retries: %w", err)
	}

	// Update hot cache
	if err := c.store.SetLatest(ctx, event.DeviceID, event); err != nil {
		c.logger.Warn("SetLatest failed (non-fatal)", zap.Error(err))
	}

	// Evaluate rules
	alerts := c.engine.Evaluate(event)

	for _, alert := range alerts {
		if err := c.store.WriteAlert(ctx, alert); err != nil {
			c.logger.Error("WriteAlert failed", zap.Error(err), zap.String("alert_id", alert.AlertID))
		}

		if err := c.alertPub.Publish(ctx, alert); err != nil {
			c.logger.Error("alert publish failed", zap.Error(err), zap.String("alert_id", alert.AlertID))
		}
	}

	return nil
}

func (c *Consumer) withRetry(ctx context.Context, name string, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := fn(); err != nil {
			lastErr = err
			c.logger.Warn("retrying operation",
				zap.String("operation", name),
				zap.Int("attempt", attempt+1),
				zap.Error(err),
			)
			backoff := time.Duration(1<<uint(attempt)) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			continue
		}
		return nil
	}
	return lastErr
}
