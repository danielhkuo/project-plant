//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/services/processor/internal/alert"
	"github.com/danielkuo/project-plant/services/processor/internal/consumer"
	"github.com/danielkuo/project-plant/services/processor/internal/engine"
	"github.com/danielkuo/project-plant/services/processor/internal/store"
)

// These tests drive consumer.Process directly (no Kafka broker) against real
// Postgres + Redis, validating cross-store behaviour deterministically.

func processEvent(t *testing.T, c *consumer.Consumer, ev any) {
	t.Helper()
	data, err := json.Marshal(ev)
	require.NoError(t, err)
	require.NoError(t, c.Process(context.Background(), data))
}

func TestRedisAndPostgres_Consistency(t *testing.T) {
	pool, cleanPG := setupPostgres(t)
	defer cleanPG()
	rdb, cleanRedis := setupRedis(t)
	defer cleanRedis()

	st := standardStore(pool, rdb)
	c := consumer.NewConsumer(engine.NewDefaultRuleEngine(), st, alert.NewRedisAlertPublisher(redisPub{rdb}), zap.NewNop())

	base := time.Now().UTC().Truncate(time.Microsecond)
	older := event("dev-consist", base)
	older.Temperature = 20
	newer := event("dev-consist", base.Add(time.Second))
	newer.Temperature = 30

	processEvent(t, c, older)
	processEvent(t, c, newer)

	// Redis latest must match the most-recent Postgres row.
	latest, err := store.NewRedisStore(rdb, time.Minute).GetLatest(context.Background(), "dev-consist")
	require.NoError(t, err)

	var pgTemp float64
	var pgRecordedAt time.Time
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT temperature, recorded_at FROM telemetry_events
		 WHERE device_id = $1 ORDER BY recorded_at DESC LIMIT 1`, "dev-consist").
		Scan(&pgTemp, &pgRecordedAt))

	assert.InDelta(t, 30.0, latest.Temperature, 0.01)
	assert.InDelta(t, pgTemp, latest.Temperature, 0.01)
	assert.WithinDuration(t, pgRecordedAt, latest.Timestamp, time.Millisecond)
}

func TestAlertStoredInBothStores(t *testing.T) {
	pool, cleanPG := setupPostgres(t)
	defer cleanPG()
	rdb, cleanRedis := setupRedis(t)
	defer cleanRedis()

	st := standardStore(pool, rdb)
	c := consumer.NewConsumer(engine.NewDefaultRuleEngine(), st, alert.NewRedisAlertPublisher(redisPub{rdb}), zap.NewNop())

	sub := rdb.Subscribe(context.Background(), "alerts:dev-both")
	defer sub.Close()
	ch := sub.Channel()

	ev := event("dev-both", time.Now().UTC().Truncate(time.Microsecond))
	ev.Temperature = 65 // > 60 -> critical_temperature (and > 40 high_temperature)
	processEvent(t, c, ev)

	// Postgres: alert row(s) present.
	assert.GreaterOrEqual(t, alertCount(t, pool, "dev-both"), 1)

	// Redis Pub/Sub: alert delivered.
	select {
	case msg := <-ch:
		var a engine.Alert
		require.NoError(t, json.Unmarshal([]byte(msg.Payload), &a))
		assert.Equal(t, "dev-both", a.DeviceID)
	case <-time.After(5 * time.Second):
		t.Fatal("alert not received on Redis pub/sub")
	}
}

func TestRedisDownDoesntBlockWrites(t *testing.T) {
	pool, cleanPG := setupPostgres(t)
	defer cleanPG()

	// Redis client pointed at a dead address with a short dial timeout: SetLatest
	// will fail, but the consumer treats it as non-fatal and still writes to PG.
	deadRedis := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: 200 * time.Millisecond})
	defer deadRedis.Close()

	st := store.NewCompositeStore(store.NewPostgresStore(pool), store.NewRedisStore(deadRedis, time.Minute))
	c := consumer.NewConsumer(engine.NewDefaultRuleEngine(), st, alert.NewRedisAlertPublisher(redisPub{deadRedis}), zap.NewNop())

	processEvent(t, c, event("dev-redisdown", time.Now().UTC().Truncate(time.Microsecond)))

	assert.Equal(t, 1, eventCount(t, pool, "dev-redisdown"),
		"event must be persisted to Postgres even when Redis is unavailable")
}
