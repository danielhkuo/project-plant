//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/kafka"

	"github.com/danielkuo/project-plant/pkg/telemetry"
	kafkapkg "github.com/danielkuo/project-plant/services/ingestion/internal/kafka"
)

func setupKafka(t *testing.T) (string, func()) {
	t.Helper()
	ctx := context.Background()

	kafkaContainer, err := kafka.Run(ctx, "confluentinc/confluent-local:7.5.0",
		kafka.WithClusterID("test-cluster"),
	)
	require.NoError(t, err)

	brokers, err := kafkaContainer.Brokers(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, brokers)

	cleanup := func() {
		kafkaContainer.Terminate(ctx)
	}
	return brokers[0], cleanup
}

func createTopic(t *testing.T, broker, topic string) {
	t.Helper()
	conn, err := kafkago.DialLeader(context.Background(), "tcp", broker, topic, 0)
	require.NoError(t, err)
	conn.Close()
}

func testEvent(deviceID string) telemetry.TelemetryEvent {
	return telemetry.TelemetryEvent{
		DeviceID:     deviceID,
		Timestamp:    time.Now().UTC().Truncate(time.Millisecond),
		Temperature:  23.5,
		Humidity:     62.3,
		SoilMoisture: 45.1,
	}
}

func readMessages(t *testing.T, broker, topic string, count int, timeout time.Duration) []kafkago.Message {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  []string{broker},
		Topic:    topic,
		MinBytes: 1,
		MaxBytes: 10e6,
		MaxWait:  500 * time.Millisecond,
	})
	defer reader.Close()

	var msgs []kafkago.Message
	for i := 0; i < count; i++ {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			break
		}
		msgs = append(msgs, msg)
	}
	return msgs
}

func TestProducer_PublishAndConsume(t *testing.T) {
	broker, cleanup := setupKafka(t)
	defer cleanup()

	topic := "test-publish-consume"
	createTopic(t, broker, topic)

	cfg := kafkapkg.ProducerConfig{
		Brokers:      []string{broker},
		Topic:        topic,
		BatchSize:    1,
		BatchTimeout: time.Millisecond,
		MaxRetries:   3,
	}
	producer := kafkapkg.NewKafkaProducer(cfg)
	defer producer.Close()

	// Publish 10 events
	for i := 0; i < 10; i++ {
		err := producer.Publish(context.Background(), testEvent("dev-001"))
		require.NoError(t, err)
	}

	// Consume and verify
	msgs := readMessages(t, broker, topic, 10, 10*time.Second)
	assert.Len(t, msgs, 10)

	for _, msg := range msgs {
		var event telemetry.TelemetryEvent
		require.NoError(t, json.Unmarshal(msg.Value, &event))
		assert.Equal(t, "dev-001", event.DeviceID)
		assert.InDelta(t, 23.5, event.Temperature, 0.01)
	}
}

func TestProducer_MessageOrdering(t *testing.T) {
	broker, cleanup := setupKafka(t)
	defer cleanup()

	topic := "test-ordering"
	createTopic(t, broker, topic)

	cfg := kafkapkg.ProducerConfig{
		Brokers:      []string{broker},
		Topic:        topic,
		BatchSize:    1,
		BatchTimeout: time.Millisecond,
		MaxRetries:   3,
	}
	producer := kafkapkg.NewKafkaProducer(cfg)
	defer producer.Close()

	// Publish events with sequential temperatures
	for i := 0; i < 10; i++ {
		event := testEvent("dev-order")
		event.Temperature = float64(i)
		require.NoError(t, producer.Publish(context.Background(), event))
	}

	msgs := readMessages(t, broker, topic, 10, 10*time.Second)
	require.Len(t, msgs, 10)

	// Same partition key = same partition = ordered
	for i, msg := range msgs {
		var event telemetry.TelemetryEvent
		require.NoError(t, json.Unmarshal(msg.Value, &event))
		assert.Equal(t, float64(i), event.Temperature, "message %d out of order", i)
	}
}

func TestProducer_UsesDeviceIDAsKey(t *testing.T) {
	broker, cleanup := setupKafka(t)
	defer cleanup()

	topic := "test-key"
	createTopic(t, broker, topic)

	cfg := kafkapkg.ProducerConfig{
		Brokers:      []string{broker},
		Topic:        topic,
		BatchSize:    1,
		BatchTimeout: time.Millisecond,
		MaxRetries:   3,
	}
	producer := kafkapkg.NewKafkaProducer(cfg)
	defer producer.Close()

	require.NoError(t, producer.Publish(context.Background(), testEvent("dev-key-001")))

	msgs := readMessages(t, broker, topic, 1, 10*time.Second)
	require.Len(t, msgs, 1)
	assert.Equal(t, "dev-key-001", string(msgs[0].Key))
}

func TestProducer_HighThroughput(t *testing.T) {
	broker, cleanup := setupKafka(t)
	defer cleanup()

	topic := "test-throughput"
	createTopic(t, broker, topic)

	cfg := kafkapkg.ProducerConfig{
		Brokers:      []string{broker},
		Topic:        topic,
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond,
		MaxRetries:   3,
	}
	producer := kafkapkg.NewKafkaProducer(cfg)
	defer producer.Close()

	count := 1000
	start := time.Now()

	for i := 0; i < count; i++ {
		event := testEvent("dev-perf")
		event.Temperature = float64(i % 80)
		err := producer.Publish(context.Background(), event)
		require.NoError(t, err)
	}

	elapsed := time.Since(start)
	throughput := float64(count) / elapsed.Seconds()

	t.Logf("Published %d events in %v (%.0f events/sec)", count, elapsed, throughput)
	// Testcontainer throughput is lower than production; validate basic functionality.
	// Production target is 500+/sec — verified via load tests (k6/vegeta) against docker-compose infra.
	assert.Greater(t, throughput, 50.0, "throughput should be > 50 events/sec even in test environment")

	// Verify all consumed
	msgs := readMessages(t, broker, topic, count, 30*time.Second)
	assert.Len(t, msgs, count)
}

func TestProducer_BrokerDown_GracefulError(t *testing.T) {
	// Use a non-existent broker address
	cfg := kafkapkg.ProducerConfig{
		Brokers:      []string{"localhost:19092"},
		Topic:        "test-down",
		BatchSize:    1,
		BatchTimeout: time.Millisecond,
		MaxRetries:   1,
	}
	producer := kafkapkg.NewKafkaProducer(cfg)
	defer producer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := producer.Publish(ctx, testEvent("dev-down"))
	assert.Error(t, err, "should return error when broker is down, not panic")
}
