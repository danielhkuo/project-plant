package store

import (
	"context"

	"github.com/danielkuo/project-plant/pkg/telemetry"
	"github.com/danielkuo/project-plant/services/processor/internal/engine"
)

// CompositeStore wires durable Postgres storage together with the Redis hot
// cache so it satisfies consumer.EventStore (Write + SetLatest + WriteAlert).
//
// Write and WriteAlert are durable (Postgres); SetLatest updates the hot cache
// (Redis). The consumer treats SetLatest failures as non-fatal, so a Redis
// outage degrades gracefully without blocking Postgres writes.
type CompositeStore struct {
	pg    *PostgresStore
	redis *RedisStore
}

// NewCompositeStore combines a Postgres durable store and a Redis hot cache.
func NewCompositeStore(pg *PostgresStore, redis *RedisStore) *CompositeStore {
	return &CompositeStore{pg: pg, redis: redis}
}

// Write persists an event to Postgres (deduplicated).
func (s *CompositeStore) Write(ctx context.Context, event telemetry.TelemetryEvent) error {
	return s.pg.Write(ctx, event)
}

// SetLatest updates the Redis hot cache with the latest reading for a device.
func (s *CompositeStore) SetLatest(ctx context.Context, deviceID string, event telemetry.TelemetryEvent) error {
	return s.redis.SetLatest(ctx, deviceID, event)
}

// WriteAlert persists an alert to Postgres.
func (s *CompositeStore) WriteAlert(ctx context.Context, alert engine.Alert) error {
	return s.pg.WriteAlert(ctx, alert)
}
