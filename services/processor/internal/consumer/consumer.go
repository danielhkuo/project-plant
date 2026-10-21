package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/pkg/telemetry"
	"github.com/danielkuo/project-plant/services/processor/internal/engine"
)

// Result label values reported to Metrics.EventProcessed.
const (
	ResultSuccess = "success"
	ResultError   = "error"
)

// Consumer processes telemetry messages from Kafka.
type Consumer struct {
	engine   *engine.RuleEngine
	store    EventStore
	alertPub AlertPublisher
	metrics  Metrics
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

// WithMetrics attaches an observer for processing outcomes. Metrics are
// optional; without this call the consumer records nothing. Attach before the
// read loop starts — the field is not synchronized with Process.
func (c *Consumer) WithMetrics(m Metrics) *Consumer {
	c.metrics = m
	return c
}

// Process handles a single raw message from Kafka in one attempt.
// Returns nil if the message was processed (or skipped if malformed).
// Returns an error if the durable write fails; Run decides whether to retry.
func (c *Consumer) Process(ctx context.Context, msg []byte) error {
	var event telemetry.TelemetryEvent
	if err := json.Unmarshal(msg, &event); err != nil {
		c.logger.Error("skipping malformed message", zap.Error(err))
		c.eventProcessed(ResultError)
		return nil // skip malformed, don't retry
	}

	// Write to durable storage. On failure Run retries the whole message, so
	// the outcome is counted once, when it is final.
	if err := c.store.Write(ctx, event); err != nil {
		return fmt.Errorf("store.Write: %w", err)
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

		if c.metrics != nil {
			c.metrics.AlertFired(alert.RuleName, alert.Severity)
		}
	}

	c.eventProcessed(ResultSuccess)
	return nil
}

func (c *Consumer) eventProcessed(result string) {
	if c.metrics != nil {
		c.metrics.EventProcessed(result)
	}
}
