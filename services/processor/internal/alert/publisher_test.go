package alert_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/danielkuo/project-plant/pkg/telemetry"
	"github.com/danielkuo/project-plant/services/processor/internal/alert"
	"github.com/danielkuo/project-plant/services/processor/internal/engine"
)

type mockRedisClient struct {
	mock.Mock
	lastChannel string
	lastMessage interface{}
}

func (m *mockRedisClient) Publish(ctx context.Context, channel string, message interface{}) error {
	m.lastChannel = channel
	m.lastMessage = message
	args := m.Called(ctx, channel, message)
	return args.Error(0)
}

func testAlert() engine.Alert {
	return engine.Alert{
		AlertID:     "550e8400-e29b-41d4-a716-446655440000",
		DeviceID:    "dev-001",
		RuleName:    "high_temperature",
		Severity:    "warning",
		TriggeredAt: time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC),
		Reading: telemetry.TelemetryEvent{
			DeviceID:     "dev-001",
			Timestamp:    time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC),
			Temperature:  50,
			Humidity:     60,
			SoilMoisture: 45,
		},
	}
}

func TestPublisher_PublishesToRedis(t *testing.T) {
	client := new(mockRedisClient)
	client.On("Publish", mock.Anything, "alerts:dev-001", mock.Anything).Return(nil)

	pub := alert.NewRedisAlertPublisher(client)
	err := pub.Publish(context.Background(), testAlert())

	require.NoError(t, err)
	assert.Equal(t, "alerts:dev-001", client.lastChannel)
}

func TestPublisher_AlertPayloadFormat(t *testing.T) {
	client := new(mockRedisClient)
	client.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	pub := alert.NewRedisAlertPublisher(client)
	err := pub.Publish(context.Background(), testAlert())
	require.NoError(t, err)

	// The message should be valid JSON with all required fields
	msgBytes, ok := client.lastMessage.([]byte)
	require.True(t, ok, "message should be []byte")

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(msgBytes, &parsed))

	assert.Contains(t, parsed, "alert_id")
	assert.Contains(t, parsed, "device_id")
	assert.Contains(t, parsed, "rule_name")
	assert.Contains(t, parsed, "severity")
	assert.Contains(t, parsed, "triggered_at")
	assert.Contains(t, parsed, "reading")
}

func TestPublisher_AlertID_IsUUID(t *testing.T) {
	client := new(mockRedisClient)
	client.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	pub := alert.NewRedisAlertPublisher(client)
	a := testAlert()
	err := pub.Publish(context.Background(), a)
	require.NoError(t, err)

	// Verify the alert_id in the published message is a valid UUID
	msgBytes := client.lastMessage.([]byte)
	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(msgBytes, &parsed))

	alertID, ok := parsed["alert_id"].(string)
	require.True(t, ok)
	assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, alertID)
}
