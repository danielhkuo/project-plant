package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/danielkuo/project-plant/pkg/telemetry"
)

// RedisDeviceReader implements api.DeviceReader using Redis.
type RedisDeviceReader struct {
	client *redis.Client
}

// NewRedisDeviceReader creates a device reader backed by Redis.
func NewRedisDeviceReader(client *redis.Client) *RedisDeviceReader {
	return &RedisDeviceReader{client: client}
}

// GetLatest retrieves the latest reading for a device.
func (r *RedisDeviceReader) GetLatest(ctx context.Context, deviceID string) (telemetry.TelemetryEvent, error) {
	key := fmt.Sprintf("device:%s:latest", deviceID)
	data, err := r.client.Get(ctx, key).Bytes()
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
func (r *RedisDeviceReader) GetAllLatest(ctx context.Context) (map[string]telemetry.TelemetryEvent, error) {
	deviceIDs, err := r.client.SMembers(ctx, "devices:active").Result()
	if err != nil {
		return nil, err
	}

	if len(deviceIDs) == 0 {
		return map[string]telemetry.TelemetryEvent{}, nil
	}

	pipe := r.client.Pipeline()
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
			continue
		}
		var event telemetry.TelemetryEvent
		if err := json.Unmarshal(data, &event); err != nil {
			continue
		}
		result[id] = event
	}
	return result, nil
}

// Ping checks Redis connectivity.
func (r *RedisDeviceReader) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}
