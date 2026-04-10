package config

import (
	"os"

	"github.com/danielkuo/project-plant/services/ingestion/internal/auth"
	"github.com/danielkuo/project-plant/services/ingestion/internal/kafka"
)

// Config holds the full ingestion service configuration.
type Config struct {
	Addr  string
	Kafka kafka.ProducerConfig
	Auth  *auth.StaticKeyAuthenticator
}

// Load reads configuration from environment variables.
func Load() Config {
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	// Default device keys for development
	keys := map[string]auth.DeviceIdentity{
		"dev-key-001": {DeviceID: "dev-001"},
		"dev-key-002": {DeviceID: "dev-002"},
		"dev-key-003": {DeviceID: "dev-003"},
	}

	return Config{
		Addr:  addr,
		Kafka: kafka.LoadFromEnv(),
		Auth:  auth.NewStaticKeyAuthenticator(keys),
	}
}
