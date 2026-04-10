package kafka_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/danielkuo/project-plant/services/ingestion/internal/api"
	"github.com/danielkuo/project-plant/services/ingestion/internal/kafka"
)

func TestProducer_ImplementsInterface(t *testing.T) {
	// Compile-time check that KafkaProducer satisfies EventProducer
	var _ api.EventProducer = (*kafka.KafkaProducer)(nil)
}

func TestProducer_DefaultConfig(t *testing.T) {
	cfg := kafka.DefaultConfig()
	assert.Equal(t, "telemetry.events", cfg.Topic)
	assert.NotEmpty(t, cfg.Brokers)
	assert.Greater(t, cfg.BatchSize, 0)
}

func TestProducer_NewDoesNotPanic(t *testing.T) {
	cfg := kafka.DefaultConfig()
	assert.NotPanics(t, func() {
		p := kafka.NewKafkaProducer(cfg)
		p.Close()
	})
}
