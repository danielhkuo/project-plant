package alert

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/danielkuo/project-plant/services/processor/internal/engine"
)

// RedisClient abstracts the Redis publish operation.
type RedisClient interface {
	Publish(ctx context.Context, channel string, message interface{}) error
}

// RedisAlertPublisher publishes alerts to Redis Pub/Sub channels.
type RedisAlertPublisher struct {
	client RedisClient
}

// NewRedisAlertPublisher creates a new publisher with the given Redis client.
func NewRedisAlertPublisher(client RedisClient) *RedisAlertPublisher {
	return &RedisAlertPublisher{client: client}
}

// Publish sends an alert to the Redis Pub/Sub channel alerts:{device_id}.
func (p *RedisAlertPublisher) Publish(ctx context.Context, alert engine.Alert) error {
	data, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("marshal alert: %w", err)
	}

	channel := fmt.Sprintf("alerts:%s", alert.DeviceID)
	return p.client.Publish(ctx, channel, data)
}
