package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/danielkuo/project-plant/pkg/telemetry"
	"github.com/danielkuo/project-plant/services/processor/internal/engine"
)

const defaultTTL = 5 * time.Minute

// RedisStore implements hot-cache storage for latest device readings.
type RedisStore struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisStore creates a store backed by the given Redis client.
func NewRedisStore(client *redis.Client, ttl time.Duration) *RedisStore {
	if ttl == 0 {
		ttl = defaultTTL
	}
	return &RedisStore{client: client, ttl: ttl}
}

// SetLatest stores the latest reading for a device with TTL.
func (s *RedisStore) SetLatest(ctx context.Context, deviceID string, event telemetry.TelemetryEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	key := fmt.Sprintf("device:%s:latest", deviceID)
	pipe := s.client.Pipeline()
	pipe.Set(ctx, key, data, s.ttl)
	pipe.SAdd(ctx, "devices:active", deviceID)
	_, err = pipe.Exec(ctx)
	return err
}

// GetLatest retrieves the latest reading for a device.
func (s *RedisStore) GetLatest(ctx context.Context, deviceID string) (telemetry.TelemetryEvent, error) {
	key := fmt.Sprintf("device:%s:latest", deviceID)
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		return telemetry.TelemetryEvent{}, err
	}

	var event telemetry.TelemetryEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return telemetry.TelemetryEvent{}, err
	}
	return event, nil
}

// GetAllLatest retrieves the latest reading for all active devices.
func (s *RedisStore) GetAllLatest(ctx context.Context) (map[string]telemetry.TelemetryEvent, error) {
	deviceIDs, err := s.client.SMembers(ctx, "devices:active").Result()
	if err != nil {
		return nil, err
	}

	if len(deviceIDs) == 0 {
		return map[string]telemetry.TelemetryEvent{}, nil
	}

	// Pipeline GET for all devices
	pipe := s.client.Pipeline()
	cmds := make(map[string]*redis.StringCmd, len(deviceIDs))
	for _, id := range deviceIDs {
		key := fmt.Sprintf("device:%s:latest", id)
		cmds[id] = pipe.Get(ctx, key)
	}
	_, err = pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		// Pipeline may have partial errors; continue collecting results
	}

	result := make(map[string]telemetry.TelemetryEvent, len(deviceIDs))
	for id, cmd := range cmds {
		data, err := cmd.Bytes()
		if err != nil {
			continue // skip devices whose keys expired
		}
		var event telemetry.TelemetryEvent
		if err := json.Unmarshal(data, &event); err != nil {
			continue
		}
		result[id] = event
	}
	return result, nil
}

// PublishAlert sends an alert to Redis Pub/Sub.
func (s *RedisStore) PublishAlert(ctx context.Context, alert engine.Alert) error {
	data, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("marshal alert: %w", err)
	}

	channel := fmt.Sprintf("alerts:%s", alert.DeviceID)
	return s.client.Publish(ctx, channel, data).Err()
}

// SubscribeAlerts subscribes to alert channels matching a pattern.
func (s *RedisStore) SubscribeAlerts(ctx context.Context, pattern string) *redis.PubSub {
	return s.client.PSubscribe(ctx, pattern)
}
