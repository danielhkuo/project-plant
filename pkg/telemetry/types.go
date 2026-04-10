package telemetry

import (
	"fmt"
	"time"
)

// TelemetryEvent represents a single telemetry reading from a plant sensor device.
type TelemetryEvent struct {
	DeviceID     string    `json:"device_id"`
	Timestamp    time.Time `json:"timestamp"`
	Temperature  float64   `json:"temperature"`
	Humidity     float64   `json:"humidity"`
	SoilMoisture float64  `json:"soil_moisture"`
}

// Validate checks that all fields are within acceptable ranges.
func (e *TelemetryEvent) Validate() error {
	if e.DeviceID == "" {
		return fmt.Errorf("device_id must not be empty")
	}
	if e.Timestamp.IsZero() {
		return fmt.Errorf("timestamp must not be zero")
	}
	if e.Temperature < -40 || e.Temperature > 80 {
		return fmt.Errorf("temperature %.2f out of range [-40, 80]", e.Temperature)
	}
	if e.Humidity < 0 || e.Humidity > 100 {
		return fmt.Errorf("humidity %.2f out of range [0, 100]", e.Humidity)
	}
	if e.SoilMoisture < 0 || e.SoilMoisture > 100 {
		return fmt.Errorf("soil_moisture %.2f out of range [0, 100]", e.SoilMoisture)
	}
	return nil
}
