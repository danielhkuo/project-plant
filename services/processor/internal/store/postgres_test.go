//go:build integration

package store_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/danielkuo/project-plant/pkg/telemetry"
	"github.com/danielkuo/project-plant/services/processor/internal/engine"
	"github.com/danielkuo/project-plant/services/processor/internal/store"
)

func setupPostgres(t *testing.T) (*store.PostgresStore, func()) {
	t.Helper()
	ctx := context.Background()

	migrationsPath, err := filepath.Abs("../../../../migrations")
	require.NoError(t, err)

	pgContainer, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.WithInitScripts(), // no init scripts, we'll apply migrations
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)

	// Apply migrations manually by reading SQL files
	migrationFiles := []string{
		"001_create_telemetry.up.sql",
		"002_create_devices.up.sql",
		"003_create_alerts.up.sql",
		"004_add_dedup_constraint.up.sql",
	}
	for _, f := range migrationFiles {
		path := filepath.Join(migrationsPath, f)
		data, err := os.ReadFile(path)
		require.NoError(t, err, "reading migration %s", f)
		_, err = pool.Exec(ctx, string(data))
		require.NoError(t, err, "applying migration %s", f)
	}

	s := store.NewPostgresStore(pool)
	cleanup := func() {
		pool.Close()
		pgContainer.Terminate(ctx)
	}
	return s, cleanup
}

func testEvent(deviceID string, recordedAt time.Time) telemetry.TelemetryEvent {
	return telemetry.TelemetryEvent{
		DeviceID:     deviceID,
		Timestamp:    recordedAt,
		Temperature:  23.5,
		Humidity:     62.3,
		SoilMoisture: 45.1,
	}
}

func TestPostgres_InsertAndQuery(t *testing.T) {
	s, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	event := testEvent("dev-001", now)

	require.NoError(t, s.Write(ctx, event))

	results, err := s.QueryByDevice(ctx, "dev-001",
		store.TimeRange{From: now.Add(-time.Hour), To: now.Add(time.Hour)},
		store.Pagination{Limit: 10, Offset: 0},
	)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "dev-001", results[0].DeviceID)
	assert.InDelta(t, 23.5, results[0].Temperature, 0.01)
}

func TestPostgres_BulkInsert(t *testing.T) {
	s, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Microsecond)

	events := make([]telemetry.TelemetryEvent, 1000)
	for i := range events {
		events[i] = testEvent("dev-bulk", base.Add(time.Duration(i)*time.Second))
	}

	start := time.Now()
	require.NoError(t, s.InsertBatch(ctx, events))
	elapsed := time.Since(start)

	assert.Less(t, elapsed, time.Second, "bulk insert of 1000 events should be < 1s")

	results, err := s.QueryByDevice(ctx, "dev-bulk",
		store.TimeRange{From: base.Add(-time.Hour), To: base.Add(2 * time.Hour)},
		store.Pagination{Limit: 1100, Offset: 0},
	)
	require.NoError(t, err)
	assert.Len(t, results, 1000)
}

func TestPostgres_QueryByTimeRange(t *testing.T) {
	s, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Microsecond)

	// Insert events across 1 hour at 1-minute intervals
	for i := 0; i < 60; i++ {
		require.NoError(t, s.Write(ctx, testEvent("dev-time", base.Add(time.Duration(i)*time.Minute))))
	}

	// Query 15-minute window (minutes 10-25)
	results, err := s.QueryByDevice(ctx, "dev-time",
		store.TimeRange{
			From: base.Add(10 * time.Minute),
			To:   base.Add(25 * time.Minute),
		},
		store.Pagination{Limit: 100, Offset: 0},
	)
	require.NoError(t, err)
	assert.Len(t, results, 16) // minutes 10,11,...,25 inclusive
}

func TestPostgres_QueryPagination(t *testing.T) {
	s, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Microsecond)

	for i := 0; i < 50; i++ {
		require.NoError(t, s.Write(ctx, testEvent("dev-page", base.Add(time.Duration(i)*time.Second))))
	}

	results, err := s.QueryByDevice(ctx, "dev-page",
		store.TimeRange{From: base.Add(-time.Hour), To: base.Add(time.Hour)},
		store.Pagination{Limit: 10, Offset: 20},
	)
	require.NoError(t, err)
	assert.Len(t, results, 10)
}

func TestPostgres_MigrationsApply(t *testing.T) {
	s, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()
	// Just verify tables exist by querying them
	_, err := s.GetStats(ctx)
	require.NoError(t, err)
}

func TestPostgres_WriteAlert(t *testing.T) {
	s, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	alert := engine.Alert{
		AlertID:     "550e8400-e29b-41d4-a716-446655440000",
		DeviceID:    "dev-alert-001",
		RuleName:    "high_temperature",
		Severity:    "warning",
		TriggeredAt: now,
		Reading:     testEvent("dev-alert-001", now),
	}

	require.NoError(t, s.WriteAlert(ctx, alert))

	alerts, err := s.QueryAlerts(ctx, store.AlertFilters{DeviceID: "dev-alert-001"})
	require.NoError(t, err)
	require.Len(t, alerts, 1)
	assert.Equal(t, "high_temperature", alerts[0].RuleName)
	assert.Equal(t, "warning", alerts[0].Severity)
}

func TestPostgres_Dedup(t *testing.T) {
	s, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	event := testEvent("dev-dedup", now)

	// Insert same event twice
	require.NoError(t, s.Write(ctx, event))
	require.NoError(t, s.Write(ctx, event)) // should be ignored

	results, err := s.QueryByDevice(ctx, "dev-dedup",
		store.TimeRange{From: now.Add(-time.Hour), To: now.Add(time.Hour)},
		store.Pagination{Limit: 10, Offset: 0},
	)
	require.NoError(t, err)
	assert.Len(t, results, 1) // only one row
}

func TestPostgres_DeviceRegistration(t *testing.T) {
	s, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()
	require.NoError(t, s.RegisterDevice(ctx, "dev-reg-001"))

	// Register again — should update last_seen
	require.NoError(t, s.RegisterDevice(ctx, "dev-reg-001"))

	stats, err := s.GetStats(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, stats.DeviceCount, 1)
}

func TestPostgres_AggregateStats(t *testing.T) {
	s, cleanup := setupPostgres(t)
	defer cleanup()

	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Microsecond)

	// Register 5 devices, insert 100 events (20 per device)
	for d := 0; d < 5; d++ {
		deviceID := fmt.Sprintf("dev-stat-%03d", d)
		require.NoError(t, s.RegisterDevice(ctx, deviceID))
		for i := 0; i < 20; i++ {
			require.NoError(t, s.Write(ctx, testEvent(deviceID, base.Add(time.Duration(i)*time.Second))))
		}
	}

	stats, err := s.GetStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 5, stats.DeviceCount)
	assert.Equal(t, int64(100), stats.EventCount)
}
