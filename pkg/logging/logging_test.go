package logging_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/danielkuo/project-plant/pkg/logging"
)

// TestStructuredLogFormat validates that log output is valid JSON carrying the
// contract fields every service must emit: timestamp, level, service, msg.
func TestStructuredLogFormat(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "out.log")

	cfg := logging.Config("test-service")
	cfg.OutputPaths = []string{logFile}

	logger, err := cfg.Build()
	require.NoError(t, err)

	logger.Info("hello world")
	require.NoError(t, logger.Sync())

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	var entry map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &entry), "log line must be valid JSON")

	assert.Equal(t, "info", entry["level"])
	assert.Equal(t, "test-service", entry["service"])
	assert.Equal(t, "hello world", entry["msg"])

	ts, ok := entry["timestamp"].(string)
	require.True(t, ok, "timestamp must be a string field")
	_, err = time.Parse("2006-01-02T15:04:05.000Z0700", ts)
	assert.NoError(t, err, "timestamp must be ISO 8601")
}
