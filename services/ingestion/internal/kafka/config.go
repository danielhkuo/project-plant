package kafka

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// ProducerConfig holds Kafka producer configuration.
type ProducerConfig struct {
	Brokers      []string
	Topic        string
	BatchSize    int
	BatchTimeout time.Duration
	MaxRetries   int
}

// DefaultConfig returns sensible defaults for local development.
func DefaultConfig() ProducerConfig {
	return ProducerConfig{
		Brokers:      []string{"localhost:9092"},
		Topic:        "telemetry.events",
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond,
		MaxRetries:   3,
	}
}

// LoadFromEnv reads configuration from environment variables, falling back to defaults.
func LoadFromEnv() ProducerConfig {
	cfg := DefaultConfig()

	if brokers := os.Getenv("KAFKA_BROKERS"); brokers != "" {
		cfg.Brokers = strings.Split(brokers, ",")
	}
	if topic := os.Getenv("KAFKA_TOPIC"); topic != "" {
		cfg.Topic = topic
	}
	if bs := os.Getenv("KAFKA_BATCH_SIZE"); bs != "" {
		if n, err := strconv.Atoi(bs); err == nil {
			cfg.BatchSize = n
		}
	}
	if bt := os.Getenv("KAFKA_BATCH_TIMEOUT"); bt != "" {
		if d, err := time.ParseDuration(bt); err == nil {
			cfg.BatchTimeout = d
		}
	}

	return cfg
}
