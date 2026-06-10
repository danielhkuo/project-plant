// Package logging provides the shared zap configuration for all services.
//
// Every service logs structured JSON with a stable key set — timestamp, level,
// service, msg — so logs from the whole pipeline can be aggregated and queried
// uniformly.
package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config returns the production logging configuration for a service.
// It mirrors zap's production defaults but renames the time key to "timestamp"
// (ISO 8601) and stamps every entry with a "service" field.
func Config(service string) zap.Config {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.TimeKey = "timestamp"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.InitialFields = map[string]interface{}{"service": service}
	return cfg
}

// MustNew builds the service logger, panicking on failure (config is static,
// so a failure here is a programming error).
func MustNew(service string) *zap.Logger {
	return zap.Must(Config(service).Build())
}
