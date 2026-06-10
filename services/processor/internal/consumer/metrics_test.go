package consumer_test

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/services/processor/internal/consumer"
	"github.com/danielkuo/project-plant/services/processor/internal/engine"
	"github.com/danielkuo/project-plant/services/processor/internal/metrics"
)

func metricsConsumer(t *testing.T) (*consumer.Consumer, *metrics.Metrics) {
	t.Helper()
	store := new(mockStore)
	store.On("Write", mock.Anything, mock.Anything).Return(nil)
	store.On("SetLatest", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("WriteAlert", mock.Anything, mock.Anything).Return(nil)

	pub := new(mockAlertPub)
	pub.On("Publish", mock.Anything, mock.Anything).Return(nil)

	m := metrics.New()
	c := consumer.NewConsumer(engine.NewDefaultRuleEngine(), store, pub, zap.NewNop()).WithMetrics(m)
	return c, m
}

// TestMetrics_AlertsFired validates the roadmap criterion: after triggering 3
// alerts, alerts_fired_total == 3. alertingMessage (temp=50) fires exactly the
// high_temperature/warning rule.
func TestMetrics_AlertsFired(t *testing.T) {
	c, m := metricsConsumer(t)

	for i := 0; i < 3; i++ {
		require.NoError(t, c.Process(context.Background(), alertingMessage()))
	}

	fired := testutil.ToFloat64(m.AlertsFiredCounter("high_temperature", "warning"))
	assert.Equal(t, 3.0, fired)
}

func TestMetrics_EventsProcessed(t *testing.T) {
	c, m := metricsConsumer(t)

	require.NoError(t, c.Process(context.Background(), validMessage()))
	require.NoError(t, c.Process(context.Background(), validMessage()))
	// Malformed payload is skipped (nil error) but counts as an error outcome.
	require.NoError(t, c.Process(context.Background(), []byte("not json")))

	assert.Equal(t, 2.0, testutil.ToFloat64(m.EventsProcessedCounter(consumer.ResultSuccess)))
	assert.Equal(t, 1.0, testutil.ToFloat64(m.EventsProcessedCounter(consumer.ResultError)))
}
