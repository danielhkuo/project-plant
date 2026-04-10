package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/compress"

	"github.com/danielkuo/project-plant/pkg/telemetry"
)

// KafkaProducer implements the api.EventProducer interface using Kafka.
type KafkaProducer struct {
	writer *kafkago.Writer
}

// NewKafkaProducer creates a producer with the given configuration.
func NewKafkaProducer(cfg ProducerConfig) *KafkaProducer {
	w := &kafkago.Writer{
		Addr:         kafkago.TCP(cfg.Brokers...),
		Topic:        cfg.Topic,
		Balancer:     &kafkago.Hash{},
		BatchSize:    cfg.BatchSize,
		BatchTimeout: cfg.BatchTimeout,
		MaxAttempts:  cfg.MaxRetries,
		Compression:  compress.Snappy,
		Async:        false, // synchronous for reliable delivery
	}

	return &KafkaProducer{writer: w}
}

// Publish serializes the event to JSON and writes it to Kafka.
// Uses device_id as the partition key for ordering guarantees.
func (p *KafkaProducer) Publish(ctx context.Context, event telemetry.TelemetryEvent) error {
	value, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	msg := kafkago.Message{
		Key:   []byte(event.DeviceID),
		Value: value,
	}

	return p.writer.WriteMessages(ctx, msg)
}

// Close flushes pending writes and closes the writer.
func (p *KafkaProducer) Close() error {
	return p.writer.Close()
}
