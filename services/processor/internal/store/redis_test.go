//go:build integration

package store_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcRedis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/danielkuo/project-plant/services/processor/internal/engine"
	"github.com/danielkuo/project-plant/services/processor/internal/store"
)

func setupRedis(t *testing.T) (*store.RedisStore, *redis.Client, func()) {
	t.Helper()
	ctx := context.Background()

	redisContainer, err := tcRedis.Run(ctx, "redis:8-alpine")
	require.NoError(t, err)

	endpoint, err := redisContainer.Endpoint(ctx, "")
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{Addr: endpoint})
	require.NoError(t, client.Ping(ctx).Err())

	s := store.NewRedisStore(client, 5*time.Minute)
	cleanup := func() {
		client.Close()
		redisContainer.Terminate(ctx)
	}
	return s, client, cleanup
}

func TestRedis_SetGetLatest(t *testing.T) {
	s, _, cleanup := setupRedis(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	event := testEvent("dev-redis-001", now)

	require.NoError(t, s.SetLatest(ctx, "dev-redis-001", event))

	result, err := s.GetLatest(ctx, "dev-redis-001")
	require.NoError(t, err)
	assert.Equal(t, "dev-redis-001", result.DeviceID)
	assert.InDelta(t, 23.5, result.Temperature, 0.01)
}

func TestRedis_OverwritesPrevious(t *testing.T) {
	s, _, cleanup := setupRedis(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	event1 := testEvent("dev-redis-002", now)
	event1.Temperature = 20.0

	event2 := testEvent("dev-redis-002", now.Add(time.Second))
	event2.Temperature = 30.0

	require.NoError(t, s.SetLatest(ctx, "dev-redis-002", event1))
	require.NoError(t, s.SetLatest(ctx, "dev-redis-002", event2))

	result, err := s.GetLatest(ctx, "dev-redis-002")
	require.NoError(t, err)
	assert.InDelta(t, 30.0, result.Temperature, 0.01)
}

func TestRedis_TTL(t *testing.T) {
	s, client, cleanup := setupRedis(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	require.NoError(t, s.SetLatest(ctx, "dev-redis-ttl", testEvent("dev-redis-ttl", now)))

	ttl := client.TTL(ctx, "device:dev-redis-ttl:latest").Val()
	assert.Greater(t, ttl, time.Duration(0))
	assert.LessOrEqual(t, ttl, 5*time.Minute)
}

func TestRedis_GetAllDevices(t *testing.T) {
	s, _, cleanup := setupRedis(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	for i := 0; i < 10; i++ {
		deviceID := fmt.Sprintf("dev-all-%03d", i)
		require.NoError(t, s.SetLatest(ctx, deviceID, testEvent(deviceID, now.Add(time.Duration(i)*time.Second))))
	}

	results, err := s.GetAllLatest(ctx)
	require.NoError(t, err)
	assert.Len(t, results, 10)
}

// A device that stops reporting must fall out of devices:active. As a plain
// set it never did, so GetAllLatest paid a wasted GET for every device the
// system had ever seen.
func TestRedis_ActiveDevicesTrimmedAfterTTL(t *testing.T) {
	_, client, cleanup := setupRedis(t)
	defer cleanup()

	ctx := context.Background()
	// Short TTL so the registry window closes during the test. The trim uses
	// second granularity, so keep a full second of slack on each side.
	s := store.NewRedisStore(client, 2*time.Second)

	require.NoError(t, s.SetLatest(ctx, "dev-stale", testEvent("dev-stale", time.Now().UTC())))
	require.Equal(t, int64(1), client.ZCard(ctx, "devices:active").Val())

	// Past the window, a later write from any device trims the stale entry.
	time.Sleep(3 * time.Second)
	require.NoError(t, s.SetLatest(ctx, "dev-fresh", testEvent("dev-fresh", time.Now().UTC())))

	members, err := client.ZRange(ctx, "devices:active", 0, -1).Result()
	require.NoError(t, err)
	assert.Equal(t, []string{"dev-fresh"}, members, "stale device should be trimmed from the registry")

	results, err := s.GetAllLatest(ctx)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Contains(t, results, "dev-fresh")
}

func TestRedis_PubSubAlert(t *testing.T) {
	s, _, cleanup := setupRedis(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now().UTC().Truncate(time.Microsecond)
	alert := engine.Alert{
		AlertID:     "550e8400-e29b-41d4-a716-446655440000",
		DeviceID:    "dev-pubsub-001",
		RuleName:    "high_temperature",
		Severity:    "warning",
		TriggeredAt: now,
		Reading:     testEvent("dev-pubsub-001", now),
	}

	// Subscribe first
	sub := s.SubscribeAlerts(ctx, "alerts:dev-pubsub-001")
	defer sub.Close()

	// Wait for subscription to be active
	_, err := sub.Receive(ctx)
	require.NoError(t, err)

	// Publish alert
	require.NoError(t, s.PublishAlert(ctx, alert))

	// Receive within 1 second
	msg, err := sub.ReceiveMessage(ctx)
	require.NoError(t, err)

	var received engine.Alert
	require.NoError(t, json.Unmarshal([]byte(msg.Payload), &received))
	assert.Equal(t, "high_temperature", received.RuleName)
	assert.Equal(t, "dev-pubsub-001", received.DeviceID)
}

func TestRedis_PubSubWildcard(t *testing.T) {
	s, _, cleanup := setupRedis(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now().UTC().Truncate(time.Microsecond)

	// Subscribe to all alerts
	sub := s.SubscribeAlerts(ctx, "alerts:*")
	defer sub.Close()

	_, err := sub.Receive(ctx)
	require.NoError(t, err)

	// Publish alert for a specific device
	alert := engine.Alert{
		AlertID:     "660e8400-e29b-41d4-a716-446655440001",
		DeviceID:    "dev-wildcard-001",
		RuleName:    "dry_soil",
		Severity:    "warning",
		TriggeredAt: now,
		Reading:     testEvent("dev-wildcard-001", now),
	}
	require.NoError(t, s.PublishAlert(ctx, alert))

	msg, err := sub.ReceiveMessage(ctx)
	require.NoError(t, err)
	assert.Contains(t, msg.Channel, "dev-wildcard-001")
}
