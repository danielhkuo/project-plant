package config

import (
	"os"
	"strings"
	"time"
)

// Config holds the full processor service configuration.
type Config struct {
	KafkaBrokers []string
	KafkaTopic   string
	KafkaGroupID string

	PostgresDSN string

	RedisAddr     string
	RedisPassword string
	RedisTTL      time.Duration

	// MetricsAddr is the listen address of the admin HTTP server serving
	// /metrics and /health (the processor's main loop is a Kafka consumer,
	// not an HTTP server).
	MetricsAddr string
}

// Load reads configuration from environment variables, falling back to
// sensible local-development defaults that match docker-compose.yml.
func Load() Config {
	return Config{
		KafkaBrokers: splitCSV(getenv("KAFKA_BROKERS", "localhost:9092")),
		KafkaTopic:   getenv("KAFKA_TOPIC", "telemetry.events"),
		KafkaGroupID: getenv("KAFKA_GROUP_ID", "plant-processor"),

		PostgresDSN: getenv("POSTGRES_DSN", "postgres://plant:plant@localhost:5432/plantdb?sslmode=disable"),

		RedisAddr:     getenv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		RedisTTL:      getDuration("REDIS_TTL", 5*time.Minute),

		MetricsAddr: getenv("METRICS_ADDR", ":9091"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
