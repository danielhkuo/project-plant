package validation

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/danielkuo/project-plant/pkg/telemetry"
)

// ValidationError provides structured error information including the field name.
type ValidationError struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"error"`
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}

// rawEvent is used for initial JSON parsing before type-safe validation.
type rawEvent struct {
	DeviceID     *string  `json:"device_id"`
	Timestamp    *string  `json:"timestamp"`
	Temperature  *float64 `json:"temperature"`
	Humidity     *float64 `json:"humidity"`
	SoilMoisture *float64 `json:"soil_moisture"`
}

// Validate parses and validates a JSON payload, returning a TelemetryEvent.
func Validate(payload []byte) (telemetry.TelemetryEvent, error) {
	if payload == nil {
		return telemetry.TelemetryEvent{}, &ValidationError{Message: "request body must not be nil"}
	}

	var raw rawEvent
	if err := json.Unmarshal(payload, &raw); err != nil {
		return telemetry.TelemetryEvent{}, &ValidationError{Message: "invalid JSON: " + err.Error()}
	}

	// Check required fields
	if raw.DeviceID == nil {
		return telemetry.TelemetryEvent{}, &ValidationError{Field: "device_id", Message: "device_id is required"}
	}
	if *raw.DeviceID == "" {
		return telemetry.TelemetryEvent{}, &ValidationError{Field: "device_id", Message: "device_id must not be empty"}
	}

	if raw.Timestamp == nil {
		return telemetry.TelemetryEvent{}, &ValidationError{Field: "timestamp", Message: "timestamp is required"}
	}
	ts, err := time.Parse(time.RFC3339Nano, *raw.Timestamp)
	if err != nil {
		return telemetry.TelemetryEvent{}, &ValidationError{Field: "timestamp", Message: "timestamp must be valid ISO 8601 format"}
	}

	if raw.Temperature == nil {
		return telemetry.TelemetryEvent{}, &ValidationError{Field: "temperature", Message: "temperature is required"}
	}
	if raw.Humidity == nil {
		return telemetry.TelemetryEvent{}, &ValidationError{Field: "humidity", Message: "humidity is required"}
	}
	if raw.SoilMoisture == nil {
		return telemetry.TelemetryEvent{}, &ValidationError{Field: "soil_moisture", Message: "soil_moisture is required"}
	}

	event := telemetry.TelemetryEvent{
		DeviceID:     *raw.DeviceID,
		Timestamp:    ts,
		Temperature:  *raw.Temperature,
		Humidity:     *raw.Humidity,
		SoilMoisture: *raw.SoilMoisture,
	}

	if err := event.Validate(); err != nil {
		// Map the pkg/telemetry error to a field-specific error
		msg := err.Error()
		field := ""
		switch {
		case contains(msg, "temperature"):
			field = "temperature"
		case contains(msg, "humidity"):
			field = "humidity"
		case contains(msg, "soil_moisture"):
			field = "soil_moisture"
		}
		return telemetry.TelemetryEvent{}, &ValidationError{Field: field, Message: msg}
	}

	return event, nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
