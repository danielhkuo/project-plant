package telemetry

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTelemetryEvent_JSONRoundTrip(t *testing.T) {
	ts := time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC)
	event := TelemetryEvent{
		DeviceID:     "dev-001",
		Timestamp:    ts,
		Temperature:  25.3,
		Humidity:     60.0,
		SoilMoisture: 45.5,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var decoded TelemetryEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, event.DeviceID, decoded.DeviceID)
	assert.Equal(t, event.Temperature, decoded.Temperature)
	assert.Equal(t, event.Humidity, decoded.Humidity)
	assert.Equal(t, event.SoilMoisture, decoded.SoilMoisture)
	assert.True(t, event.Timestamp.Equal(decoded.Timestamp))
}

func TestTelemetryEvent_JSONFieldNames(t *testing.T) {
	ts := time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC)
	event := TelemetryEvent{
		DeviceID:     "dev-001",
		Timestamp:    ts,
		Temperature:  25.3,
		Humidity:     60.0,
		SoilMoisture: 45.5,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	expectedKeys := []string{"device_id", "timestamp", "temperature", "humidity", "soil_moisture"}
	for _, key := range expectedKeys {
		assert.Contains(t, raw, key, "JSON output missing key: %s", key)
	}
	assert.Len(t, raw, len(expectedKeys), "JSON output has unexpected extra keys")
}

func TestTelemetryEvent_Validate(t *testing.T) {
	tests := []struct {
		name    string
		event   TelemetryEvent
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid event",
			event: TelemetryEvent{
				DeviceID:     "dev-001",
				Timestamp:    time.Now(),
				Temperature:  25.3,
				Humidity:     60.0,
				SoilMoisture: 45.5,
			},
			wantErr: false,
		},
		{
			name: "valid boundary min",
			event: TelemetryEvent{
				DeviceID:     "x",
				Timestamp:    time.Now(),
				Temperature:  -40,
				Humidity:     0,
				SoilMoisture: 0,
			},
			wantErr: false,
		},
		{
			name: "valid boundary max",
			event: TelemetryEvent{
				DeviceID:     "dev-999",
				Timestamp:    time.Now(),
				Temperature:  80,
				Humidity:     100,
				SoilMoisture: 100,
			},
			wantErr: false,
		},
		{
			name: "empty device_id",
			event: TelemetryEvent{
				DeviceID:     "",
				Timestamp:    time.Now(),
				Temperature:  25.0,
				Humidity:     60.0,
				SoilMoisture: 45.0,
			},
			wantErr: true,
			errMsg:  "device_id",
		},
		{
			name: "zero timestamp",
			event: TelemetryEvent{
				DeviceID:     "dev-001",
				Timestamp:    time.Time{},
				Temperature:  25.0,
				Humidity:     60.0,
				SoilMoisture: 45.0,
			},
			wantErr: true,
			errMsg:  "timestamp",
		},
		{
			name: "temperature too low",
			event: TelemetryEvent{
				DeviceID:     "dev-001",
				Timestamp:    time.Now(),
				Temperature:  -41,
				Humidity:     60.0,
				SoilMoisture: 45.0,
			},
			wantErr: true,
			errMsg:  "temperature",
		},
		{
			name: "temperature too high",
			event: TelemetryEvent{
				DeviceID:     "dev-001",
				Timestamp:    time.Now(),
				Temperature:  81,
				Humidity:     60.0,
				SoilMoisture: 45.0,
			},
			wantErr: true,
			errMsg:  "temperature",
		},
		{
			name: "humidity below zero",
			event: TelemetryEvent{
				DeviceID:     "dev-001",
				Timestamp:    time.Now(),
				Temperature:  25.0,
				Humidity:     -1,
				SoilMoisture: 45.0,
			},
			wantErr: true,
			errMsg:  "humidity",
		},
		{
			name: "humidity above 100",
			event: TelemetryEvent{
				DeviceID:     "dev-001",
				Timestamp:    time.Now(),
				Temperature:  25.0,
				Humidity:     101,
				SoilMoisture: 45.0,
			},
			wantErr: true,
			errMsg:  "humidity",
		},
		{
			name: "soil_moisture below zero",
			event: TelemetryEvent{
				DeviceID:     "dev-001",
				Timestamp:    time.Now(),
				Temperature:  25.0,
				Humidity:     60.0,
				SoilMoisture: -0.1,
			},
			wantErr: true,
			errMsg:  "soil_moisture",
		},
		{
			name: "soil_moisture above 100",
			event: TelemetryEvent{
				DeviceID:     "dev-001",
				Timestamp:    time.Now(),
				Temperature:  25.0,
				Humidity:     60.0,
				SoilMoisture: 100.1,
			},
			wantErr: true,
			errMsg:  "soil_moisture",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
