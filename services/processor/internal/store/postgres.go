package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielkuo/project-plant/pkg/telemetry"
	"github.com/danielkuo/project-plant/services/processor/internal/engine"
)

// PostgresStore implements persistent storage for telemetry events and alerts.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a store backed by the given connection pool.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// Write inserts a single telemetry event. Uses ON CONFLICT DO NOTHING for dedup.
func (s *PostgresStore) Write(ctx context.Context, event telemetry.TelemetryEvent) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO telemetry_events (device_id, recorded_at, temperature, humidity, soil_moisture)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (device_id, recorded_at) DO NOTHING`,
		event.DeviceID, event.Timestamp, event.Temperature, event.Humidity, event.SoilMoisture,
	)
	return err
}

// InsertBatch inserts multiple events efficiently using CopyFrom.
func (s *PostgresStore) InsertBatch(ctx context.Context, events []telemetry.TelemetryEvent) error {
	rows := make([][]interface{}, len(events))
	for i, e := range events {
		rows[i] = []interface{}{e.DeviceID, e.Timestamp, e.Temperature, e.Humidity, e.SoilMoisture}
	}

	_, err := s.pool.CopyFrom(
		ctx,
		pgx.Identifier{"telemetry_events"},
		[]string{"device_id", "recorded_at", "temperature", "humidity", "soil_moisture"},
		pgx.CopyFromRows(rows),
	)
	return err
}

// TimeRange specifies a time window for queries.
type TimeRange struct {
	From time.Time
	To   time.Time
}

// Pagination specifies limit and offset for queries.
type Pagination struct {
	Limit  int
	Offset int
}

// QueryByDevice retrieves historical telemetry for a device within a time range.
func (s *PostgresStore) QueryByDevice(ctx context.Context, deviceID string, tr TimeRange, pg Pagination) ([]telemetry.TelemetryEvent, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT device_id, recorded_at, temperature, humidity, soil_moisture
		 FROM telemetry_events
		 WHERE device_id = $1 AND recorded_at >= $2 AND recorded_at <= $3
		 ORDER BY recorded_at DESC
		 LIMIT $4 OFFSET $5`,
		deviceID, tr.From, tr.To, pg.Limit, pg.Offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []telemetry.TelemetryEvent
	for rows.Next() {
		var e telemetry.TelemetryEvent
		if err := rows.Scan(&e.DeviceID, &e.Timestamp, &e.Temperature, &e.Humidity, &e.SoilMoisture); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// WriteAlert inserts an alert record.
func (s *PostgresStore) WriteAlert(ctx context.Context, alert engine.Alert) error {
	readingJSON, err := json.Marshal(alert.Reading)
	if err != nil {
		return fmt.Errorf("marshal reading: %w", err)
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO alerts (id, device_id, rule_name, severity, triggered_at, reading)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		alert.AlertID, alert.DeviceID, alert.RuleName, alert.Severity, alert.TriggeredAt, readingJSON,
	)
	return err
}

// AlertFilters defines query filters for alerts.
type AlertFilters struct {
	DeviceID string
	Severity string
	Status   string // "active" or "resolved"
}

// QueryAlerts retrieves alerts matching the given filters.
func (s *PostgresStore) QueryAlerts(ctx context.Context, filters AlertFilters) ([]engine.Alert, error) {
	query := `SELECT id, device_id, rule_name, severity, triggered_at, reading
		FROM alerts WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if filters.DeviceID != "" {
		query += fmt.Sprintf(" AND device_id = $%d", argIdx)
		args = append(args, filters.DeviceID)
		argIdx++
	}
	if filters.Severity != "" {
		query += fmt.Sprintf(" AND severity = $%d", argIdx)
		args = append(args, filters.Severity)
		argIdx++
	}
	if filters.Status == "active" {
		query += " AND resolved_at IS NULL"
	} else if filters.Status == "resolved" {
		query += " AND resolved_at IS NOT NULL"
	}

	query += " ORDER BY triggered_at DESC"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []engine.Alert
	for rows.Next() {
		var a engine.Alert
		var readingJSON []byte
		if err := rows.Scan(&a.AlertID, &a.DeviceID, &a.RuleName, &a.Severity, &a.TriggeredAt, &readingJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(readingJSON, &a.Reading); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

// RegisterDevice inserts or updates a device record.
func (s *PostgresStore) RegisterDevice(ctx context.Context, deviceID string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO devices (device_id) VALUES ($1)
		 ON CONFLICT (device_id) DO UPDATE SET last_seen = NOW()`,
		deviceID,
	)
	return err
}

// DeviceStats holds aggregate fleet statistics.
type DeviceStats struct {
	DeviceCount  int
	EventCount   int64
	ActiveAlerts int
}

// GetStats returns aggregate fleet statistics.
func (s *PostgresStore) GetStats(ctx context.Context) (DeviceStats, error) {
	var stats DeviceStats

	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM devices`).Scan(&stats.DeviceCount)
	if err != nil {
		return stats, err
	}

	err = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM telemetry_events`).Scan(&stats.EventCount)
	if err != nil {
		return stats, err
	}

	err = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM alerts WHERE resolved_at IS NULL`).Scan(&stats.ActiveAlerts)
	return stats, err
}
