package validation_test

import (
	"testing"

	"github.com/danielkuo/project-plant/services/ingestion/internal/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validPayload() []byte {
	return []byte(`{
		"device_id": "dev-001",
		"timestamp": "2026-04-09T12:00:00Z",
		"temperature": 23.5,
		"humidity": 62.3,
		"soil_moisture": 45.1
	}`)
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		wantErr   bool
		errField  string
		errSubstr string
	}{
		{
			name:  "valid payload",
			input: validPayload(),
		},
		{
			name:      "missing device_id",
			input:     []byte(`{"timestamp":"2026-04-09T12:00:00Z","temperature":23.5,"humidity":62.3,"soil_moisture":45.1}`),
			wantErr:   true,
			errField:  "device_id",
			errSubstr: "device_id",
		},
		{
			name:      "empty device_id",
			input:     []byte(`{"device_id":"","timestamp":"2026-04-09T12:00:00Z","temperature":23.5,"humidity":62.3,"soil_moisture":45.1}`),
			wantErr:   true,
			errField:  "device_id",
			errSubstr: "device_id",
		},
		{
			name:      "temperature too low",
			input:     []byte(`{"device_id":"dev-001","timestamp":"2026-04-09T12:00:00Z","temperature":-41,"humidity":62.3,"soil_moisture":45.1}`),
			wantErr:   true,
			errField:  "temperature",
			errSubstr: "temperature",
		},
		{
			name:      "temperature too high",
			input:     []byte(`{"device_id":"dev-001","timestamp":"2026-04-09T12:00:00Z","temperature":81,"humidity":62.3,"soil_moisture":45.1}`),
			wantErr:   true,
			errField:  "temperature",
			errSubstr: "temperature",
		},
		{
			name:      "humidity below zero",
			input:     []byte(`{"device_id":"dev-001","timestamp":"2026-04-09T12:00:00Z","temperature":23.5,"humidity":-1,"soil_moisture":45.1}`),
			wantErr:   true,
			errField:  "humidity",
			errSubstr: "humidity",
		},
		{
			name:      "humidity above 100",
			input:     []byte(`{"device_id":"dev-001","timestamp":"2026-04-09T12:00:00Z","temperature":23.5,"humidity":101,"soil_moisture":45.1}`),
			wantErr:   true,
			errField:  "humidity",
			errSubstr: "humidity",
		},
		{
			name:      "soil moisture below zero",
			input:     []byte(`{"device_id":"dev-001","timestamp":"2026-04-09T12:00:00Z","temperature":23.5,"humidity":62.3,"soil_moisture":-0.1}`),
			wantErr:   true,
			errField:  "soil_moisture",
			errSubstr: "soil_moisture",
		},
		{
			name:      "soil moisture above 100",
			input:     []byte(`{"device_id":"dev-001","timestamp":"2026-04-09T12:00:00Z","temperature":23.5,"humidity":62.3,"soil_moisture":100.1}`),
			wantErr:   true,
			errField:  "soil_moisture",
			errSubstr: "soil_moisture",
		},
		{
			name:      "missing timestamp",
			input:     []byte(`{"device_id":"dev-001","temperature":23.5,"humidity":62.3,"soil_moisture":45.1}`),
			wantErr:   true,
			errField:  "timestamp",
			errSubstr: "timestamp",
		},
		{
			name:      "invalid timestamp format",
			input:     []byte(`{"device_id":"dev-001","timestamp":"not-a-date","temperature":23.5,"humidity":62.3,"soil_moisture":45.1}`),
			wantErr:   true,
			errField:  "timestamp",
			errSubstr: "timestamp",
		},
		{
			name:  "extra fields allowed",
			input: []byte(`{"device_id":"dev-001","timestamp":"2026-04-09T12:00:00Z","temperature":23.5,"humidity":62.3,"soil_moisture":45.1,"extra":"field"}`),
		},
		{
			name:      "null body",
			input:     nil,
			wantErr:   true,
			errSubstr: "body",
		},
		{
			name:      "malformed JSON",
			input:     []byte(`{broken`),
			wantErr:   true,
			errSubstr: "JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := validation.Validate(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				if tt.errField != "" {
					valErr, ok := err.(*validation.ValidationError)
					require.True(t, ok, "error should be *ValidationError")
					assert.Equal(t, tt.errField, valErr.Field)
				}
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, event.DeviceID)
				assert.False(t, event.Timestamp.IsZero())
			}
		})
	}
}
