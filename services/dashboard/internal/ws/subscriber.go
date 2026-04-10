package ws

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/pkg/telemetry"
)

// RedisSubscriber bridges Redis Pub/Sub channels into the WebSocket Hub.
type RedisSubscriber struct {
	client *redis.Client
	hub    *Hub
	logger *zap.Logger
}

// NewRedisSubscriber creates a subscriber that forwards Redis events to the Hub.
func NewRedisSubscriber(client *redis.Client, hub *Hub, logger *zap.Logger) *RedisSubscriber {
	return &RedisSubscriber{client: client, hub: hub, logger: logger}
}

// Run subscribes to Redis channels and forwards messages to the Hub.
// Blocks until ctx is cancelled.
func (s *RedisSubscriber) Run(ctx context.Context) error {
	pubsub := s.client.PSubscribe(ctx, "alerts:*", "readings:*")
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case redisMsg, ok := <-ch:
			if !ok {
				return nil
			}
			msg, err := s.parseMessage(redisMsg)
			if err != nil {
				s.logger.Warn("failed to parse pubsub message",
					zap.String("channel", redisMsg.Channel),
					zap.Error(err),
				)
				continue
			}
			s.hub.Broadcast(msg)

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *RedisSubscriber) parseMessage(redisMsg *redis.Message) (Message, error) {
	channel := redisMsg.Channel

	if strings.HasPrefix(channel, "readings:") {
		deviceID := strings.TrimPrefix(channel, "readings:")
		var event telemetry.TelemetryEvent
		if err := json.Unmarshal([]byte(redisMsg.Payload), &event); err != nil {
			return Message{}, err
		}
		return Message{
			Type:     MessageTypeReading,
			DeviceID: deviceID,
			Payload:  event,
		}, nil
	}

	if strings.HasPrefix(channel, "alerts:") {
		deviceID := strings.TrimPrefix(channel, "alerts:")
		var payload json.RawMessage
		if err := json.Unmarshal([]byte(redisMsg.Payload), &payload); err != nil {
			return Message{}, err
		}
		return Message{
			Type:     MessageTypeAlert,
			DeviceID: deviceID,
			Payload:  payload,
		}, nil
	}

	return Message{}, nil
}
