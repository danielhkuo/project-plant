package config

import "os"

// Config holds the dashboard service configuration.
type Config struct {
	Addr        string
	PostgresDSN string
	RedisAddr   string
	CORSOrigin  string
}

// Load reads configuration from environment variables.
func Load() Config {
	return Config{
		Addr:        envOrDefault("LISTEN_ADDR", ":8081"),
		PostgresDSN: envOrDefault("POSTGRES_DSN", "postgres://plant:plant@localhost:5432/plantdb?sslmode=disable"),
		RedisAddr:   envOrDefault("REDIS_ADDR", "localhost:6379"),
		CORSOrigin:  envOrDefault("CORS_ORIGIN", "http://localhost:3000"),
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
