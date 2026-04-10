package consumer_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/pkg/telemetry"
	"github.com/danielkuo/project-plant/services/processor/internal/consumer"
	"github.com/danielkuo/project-plant/services/processor/internal/engine"
)

// --- Mocks ---

type mockStore struct {
	mock.Mock
}

func (m *mockStore) Write(ctx context.Context, event telemetry.TelemetryEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *mockStore) SetLatest(ctx context.Context, deviceID string, event telemetry.TelemetryEvent) error {
	args := m.Called(ctx, deviceID, event)
	return args.Error(0)
}

func (m *mockStore) WriteAlert(ctx context.Context, alert engine.Alert) error {
	args := m.Called(ctx, alert)
	return args.Error(0)
}

type mockAlertPub struct {
	mock.Mock
}

func (m *mockAlertPub) Publish(ctx context.Context, alert engine.Alert) error {
	args := m.Called(ctx, alert)
	return args.Error(0)
}

func validMessage() []byte {
	event := telemetry.TelemetryEvent{
		DeviceID:     "dev-001",
		Timestamp:    time.Now().UTC(),
		Temperature:  25,
		Humidity:     60,
		SoilMoisture: 45,
	}
	b, _ := json.Marshal(event)
	return b
}

func alertingMessage() []byte {
	event := telemetry.TelemetryEvent{
		DeviceID:     "dev-001",
		Timestamp:    time.Now().UTC(),
		Temperature:  50, // triggers high_temperature
		Humidity:     60,
		SoilMoisture: 45,
	}
	b, _ := json.Marshal(event)
	return b
}

// --- Tests ---

func TestConsumer_ProcessesMessage(t *testing.T) {
	store := new(mockStore)
	pub := new(mockAlertPub)
	eng := engine.NewDefaultRuleEngine()

	store.On("Write", mock.Anything, mock.Anything).Return(nil)
	store.On("SetLatest", mock.Anything, "dev-001", mock.Anything).Return(nil)

	c := consumer.NewConsumer(eng, store, pub, zap.NewNop())
	err := c.Process(context.Background(), validMessage())

	require.NoError(t, err)
	store.AssertCalled(t, "Write", mock.Anything, mock.Anything)
	store.AssertCalled(t, "SetLatest", mock.Anything, "dev-001", mock.Anything)
	pub.AssertNotCalled(t, "Publish") // no alerts for normal reading
}

func TestConsumer_AlertTriggered(t *testing.T) {
	store := new(mockStore)
	pub := new(mockAlertPub)
	eng := engine.NewDefaultRuleEngine()

	store.On("Write", mock.Anything, mock.Anything).Return(nil)
	store.On("SetLatest", mock.Anything, "dev-001", mock.Anything).Return(nil)
	store.On("WriteAlert", mock.Anything, mock.Anything).Return(nil)
	pub.On("Publish", mock.Anything, mock.Anything).Return(nil)

	c := consumer.NewConsumer(eng, store, pub, zap.NewNop())
	err := c.Process(context.Background(), alertingMessage())

	require.NoError(t, err)
	store.AssertCalled(t, "WriteAlert", mock.Anything, mock.Anything)
	pub.AssertCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestConsumer_SkipsMalformed(t *testing.T) {
	store := new(mockStore)
	pub := new(mockAlertPub)
	eng := engine.NewDefaultRuleEngine()

	c := consumer.NewConsumer(eng, store, pub, zap.NewNop())
	err := c.Process(context.Background(), []byte(`{broken json`))

	// Should not error — malformed messages are skipped
	require.NoError(t, err)
	store.AssertNotCalled(t, "Write")
}

func TestConsumer_CommitsOffset(t *testing.T) {
	// Process returns nil on success, meaning the consumer loop should commit
	store := new(mockStore)
	pub := new(mockAlertPub)
	eng := engine.NewDefaultRuleEngine()

	store.On("Write", mock.Anything, mock.Anything).Return(nil)
	store.On("SetLatest", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	c := consumer.NewConsumer(eng, store, pub, zap.NewNop())
	err := c.Process(context.Background(), validMessage())

	assert.NoError(t, err, "nil return signals offset should be committed")
}

func TestConsumer_StoreError_Retries(t *testing.T) {
	store := new(mockStore)
	pub := new(mockAlertPub)
	eng := engine.NewDefaultRuleEngine()

	// Fail twice, succeed on third
	store.On("Write", mock.Anything, mock.Anything).
		Return(errors.New("db connection lost")).Times(2)
	store.On("Write", mock.Anything, mock.Anything).
		Return(nil).Once()
	store.On("SetLatest", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	c := consumer.NewConsumer(eng, store, pub, zap.NewNop())
	err := c.Process(context.Background(), validMessage())

	require.NoError(t, err)
	// 3 Write calls (2 failed + 1 success) + 1 SetLatest = 4 total
	store.AssertNumberOfCalls(t, "Write", 3)
	store.AssertCalled(t, "SetLatest", mock.Anything, mock.Anything, mock.Anything)
}
