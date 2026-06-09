//go:build integration

// Package integration_test exercises the full processing pipeline
// (Kafka -> Consumer -> Postgres/Redis + alerts) against real infrastructure
// spun up with testcontainers. Run with: go test -tags=integration ./integration_test/...
package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcKafka "github.com/testcontainers/testcontainers-go/modules/kafka"
	tcPostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcRedis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/pkg/telemetry"
	"github.com/danielkuo/project-plant/services/processor/internal/alert"
	"github.com/danielkuo/project-plant/services/processor/internal/consumer"
	"github.com/danielkuo/project-plant/services/processor/internal/engine"
	"github.com/danielkuo/project-plant/services/processor/internal/store"
)

// ---- infrastructure helpers (mirror the repo's existing testcontainers setup) ----

func setupKafka(t *testing.T) (string, func()) {
	t.Helper()
	ctx := context.Background()
	c, err := tcKafka.Run(ctx, "confluentinc/confluent-local:7.5.0", tcKafka.WithClusterID("test-cluster"))
	require.NoError(t, err)
	brokers, err := c.Brokers(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, brokers)
	return brokers[0], func() { c.Terminate(ctx) }
}

func createTopic(t *testing.T, broker, topic string) {
	t.Helper()
	conn, err := kafkago.DialLeader(context.Background(), "tcp", broker, topic, 0)
	require.NoError(t, err)
	conn.Close()
}

func setupPostgres(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()

	migrationsPath, err := filepath.Abs("../../../migrations")
	require.NoError(t, err)

	c, err := tcPostgres.Run(ctx, "postgres:17-alpine",
		tcPostgres.WithDatabase("testdb"),
		tcPostgres.WithUsername("test"),
		tcPostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	connStr, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)

	for _, f := range []string{
		"001_create_telemetry.up.sql",
		"002_create_devices.up.sql",
		"003_create_alerts.up.sql",
		"004_add_dedup_constraint.up.sql",
	} {
		data, err := os.ReadFile(filepath.Join(migrationsPath, f))
		require.NoError(t, err, "reading migration %s", f)
		_, err = pool.Exec(ctx, string(data))
		require.NoError(t, err, "applying migration %s", f)
	}

	return pool, func() {
		pool.Close()
		c.Terminate(ctx)
	}
}

func setupRedis(t *testing.T) (*redis.Client, func()) {
	t.Helper()
	ctx := context.Background()
	c, err := tcRedis.Run(ctx, "redis:8-alpine")
	require.NoError(t, err)
	endpoint, err := c.Endpoint(ctx, "")
	require.NoError(t, err)
	client := redis.NewClient(&redis.Options{Addr: endpoint})
	require.NoError(t, client.Ping(ctx).Err())
	return client, func() {
		client.Close()
		c.Terminate(ctx)
	}
}

// redisPub adapts *redis.Client to alert.RedisClient (Publish returning error),
// mirroring the adapter in cmd/processor/main.go.
type redisPub struct{ c *redis.Client }

func (r redisPub) Publish(ctx context.Context, channel string, message interface{}) error {
	return r.c.Publish(ctx, channel, message).Err()
}

func standardStore(pool *pgxpool.Pool, rdb *redis.Client) *store.CompositeStore {
	return store.NewCompositeStore(store.NewPostgresStore(pool), store.NewRedisStore(rdb, time.Minute))
}

// ---- pipeline harness ----

type infra struct {
	broker string
	pool   *pgxpool.Pool
	redis  *redis.Client
}

// newInfra spins up Kafka + Postgres + Redis and registers cleanup.
func newInfra(t *testing.T) *infra {
	t.Helper()
	broker, cleanKafka := setupKafka(t)
	pool, cleanPG := setupPostgres(t)
	rdb, cleanRedis := setupRedis(t)
	t.Cleanup(func() { cleanRedis(); cleanPG(); cleanKafka() })
	return &infra{broker: broker, pool: pool, redis: rdb}
}

type pipeline struct {
	*infra
	topic  string
	writer *kafkago.Writer
}

// startPipeline creates a topic, runs the background read loop (mirroring
// cmd/processor/main.go) wired to the given store/publisher, and returns a
// producer handle.
func startPipeline(t *testing.T, in *infra, st consumer.EventStore, alertPub consumer.AlertPublisher) *pipeline {
	t.Helper()
	topic := fmt.Sprintf("telemetry.events.%d", time.Now().UnixNano())
	createTopic(t, in.broker, topic)

	c := consumer.NewConsumer(engine.NewDefaultRuleEngine(), st, alertPub, zap.NewNop())
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:     []string{in.broker},
		Topic:       topic,
		GroupID:     "plant-processor-" + topic,
		StartOffset: kafkago.FirstOffset,
	})
	ctx, cancel := context.WithCancel(context.Background())
	go runLoop(ctx, reader, c)
	t.Cleanup(func() { cancel(); reader.Close() })

	writer := &kafkago.Writer{Addr: kafkago.TCP(in.broker), Topic: topic, BatchTimeout: 5 * time.Millisecond}
	t.Cleanup(func() { writer.Close() })

	return &pipeline{infra: in, topic: topic, writer: writer}
}

// newStandardPipeline builds infra + the production composite store/publisher.
func newStandardPipeline(t *testing.T) *pipeline {
	in := newInfra(t)
	return startPipeline(t, in, standardStore(in.pool, in.redis), alert.NewRedisAlertPublisher(redisPub{in.redis}))
}

// runLoop mirrors the read loop in cmd/processor/main.go: commit only on success.
func runLoop(ctx context.Context, reader *kafkago.Reader, c *consumer.Consumer) {
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			return // context cancelled or reader closed
		}
		if err := c.Process(ctx, msg.Value); err != nil {
			continue // do not commit -> redelivered on next session
		}
		_ = reader.CommitMessages(ctx, msg)
	}
}

func (p *pipeline) produce(t *testing.T, events ...telemetry.TelemetryEvent) {
	t.Helper()
	msgs := make([]kafkago.Message, 0, len(events))
	for _, e := range events {
		data, err := json.Marshal(e)
		require.NoError(t, err)
		msgs = append(msgs, kafkago.Message{Key: []byte(e.DeviceID), Value: data})
	}
	require.NoError(t, p.writer.WriteMessages(context.Background(), msgs...))
}

func eventCount(t *testing.T, pool *pgxpool.Pool, deviceID string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM telemetry_events WHERE device_id = $1`, deviceID).Scan(&n))
	return n
}

func alertCount(t *testing.T, pool *pgxpool.Pool, deviceID string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM alerts WHERE device_id = $1`, deviceID).Scan(&n))
	return n
}

func event(deviceID string, at time.Time) telemetry.TelemetryEvent {
	return telemetry.TelemetryEvent{
		DeviceID: deviceID, Timestamp: at,
		Temperature: 23.5, Humidity: 62.3, SoilMoisture: 45.1,
	}
}

// eventually polls fn until it returns true or the timeout elapses.
func eventually(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}

// ---- tests ----

func TestPipeline_NormalEvent(t *testing.T) {
	p := newStandardPipeline(t)

	now := time.Now().UTC().Truncate(time.Microsecond)
	p.produce(t, event("dev-001", now))

	eventually(t, 15*time.Second, func() bool { return eventCount(t, p.pool, "dev-001") == 1 })

	latest, err := store.NewRedisStore(p.redis, time.Minute).GetLatest(context.Background(), "dev-001")
	require.NoError(t, err)
	assert.Equal(t, "dev-001", latest.DeviceID)
}

func TestPipeline_AlertTriggered(t *testing.T) {
	p := newStandardPipeline(t)

	// Subscribe before producing so we catch the published alert.
	sub := p.redis.Subscribe(context.Background(), "alerts:dev-hot")
	defer sub.Close()
	ch := sub.Channel()

	now := time.Now().UTC().Truncate(time.Microsecond)
	hot := event("dev-hot", now)
	hot.Temperature = 50 // > 40 -> high_temperature warning
	p.produce(t, hot)

	eventually(t, 15*time.Second, func() bool { return alertCount(t, p.pool, "dev-hot") >= 1 })

	select {
	case msg := <-ch:
		var a engine.Alert
		require.NoError(t, json.Unmarshal([]byte(msg.Payload), &a))
		assert.Equal(t, "dev-hot", a.DeviceID)
		assert.Equal(t, "high_temperature", a.RuleName)
	case <-time.After(10 * time.Second):
		t.Fatal("did not receive alert on Redis pub/sub")
	}
}

func TestPipeline_Idempotency(t *testing.T) {
	p := newStandardPipeline(t)

	now := time.Now().UTC().Truncate(time.Microsecond)
	e := event("dev-dup", now)
	p.produce(t, e, e) // identical (device_id, recorded_at)

	eventually(t, 15*time.Second, func() bool { return eventCount(t, p.pool, "dev-dup") >= 1 })
	time.Sleep(2 * time.Second) // give the duplicate a chance to (wrongly) land
	assert.Equal(t, 1, eventCount(t, p.pool, "dev-dup"), "duplicate event must not create a second row")
}

func TestPipeline_100Events_AllProcessed(t *testing.T) {
	p := newStandardPipeline(t)

	now := time.Now().UTC().Truncate(time.Microsecond)
	events := make([]telemetry.TelemetryEvent, 100)
	for i := 0; i < 100; i++ {
		events[i] = event("dev-bulk", now.Add(time.Duration(i)*time.Millisecond))
	}
	p.produce(t, events...)

	eventually(t, 30*time.Second, func() bool { return eventCount(t, p.pool, "dev-bulk") == 100 })

	latest, err := store.NewRedisStore(p.redis, time.Minute).GetLatest(context.Background(), "dev-bulk")
	require.NoError(t, err)
	assert.Equal(t, "dev-bulk", latest.DeviceID)
}

// failableStore wraps the real composite store and can simulate Postgres
// downtime by failing Write, exercising the "no commit -> Kafka buffers ->
// redelivery on recovery" guarantee.
type failableStore struct {
	*store.CompositeStore
	mu   sync.Mutex
	fail bool
}

func (s *failableStore) Write(ctx context.Context, e telemetry.TelemetryEvent) error {
	s.mu.Lock()
	f := s.fail
	s.mu.Unlock()
	if f {
		return fmt.Errorf("simulated postgres outage")
	}
	return s.CompositeStore.Write(ctx, e)
}

func (s *failableStore) setFail(v bool) {
	s.mu.Lock()
	s.fail = v
	s.mu.Unlock()
}

func TestPipeline_KafkaBuffering(t *testing.T) {
	in := newInfra(t)

	topic := fmt.Sprintf("telemetry.buffering.%d", time.Now().UnixNano())
	createTopic(t, in.broker, topic)
	groupID := "plant-processor-" + topic

	fs := &failableStore{CompositeStore: standardStore(in.pool, in.redis)}
	fs.setFail(true) // Postgres "down"

	c := consumer.NewConsumer(engine.NewDefaultRuleEngine(), fs, alert.NewRedisAlertPublisher(redisPub{in.redis}), zap.NewNop())

	// Produce 100 events while the store is down.
	writer := &kafkago.Writer{Addr: kafkago.TCP(in.broker), Topic: topic, BatchTimeout: 5 * time.Millisecond}
	defer writer.Close()
	now := time.Now().UTC().Truncate(time.Microsecond)
	msgs := make([]kafkago.Message, 100)
	for i := 0; i < 100; i++ {
		data, _ := json.Marshal(event("dev-buf", now.Add(time.Duration(i)*time.Millisecond)))
		msgs[i] = kafkago.Message{Key: []byte("dev-buf"), Value: data}
	}
	require.NoError(t, writer.WriteMessages(context.Background(), msgs...))

	newReader := func() *kafkago.Reader {
		return kafkago.NewReader(kafkago.ReaderConfig{
			Brokers: []string{in.broker}, Topic: topic, GroupID: groupID, StartOffset: kafkago.FirstOffset,
		})
	}

	// Session 1: store down -> every Process fails -> nothing committed.
	ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	r1 := newReader()
	runLoop(ctx1, r1, c)
	cancel1()
	r1.Close()
	require.Equal(t, 0, eventCount(t, in.pool, "dev-buf"), "no events should be stored while postgres is down")

	// Recovery: store back up. Session 2 (same group) resumes from the last
	// committed offset (none) and redelivers all 100 events -> zero data loss.
	fs.setFail(false)
	ctx2, cancel2 := context.WithCancel(context.Background())
	r2 := newReader()
	go runLoop(ctx2, r2, c)
	defer func() { cancel2(); r2.Close() }()

	eventually(t, 30*time.Second, func() bool { return eventCount(t, in.pool, "dev-buf") == 100 })
}
