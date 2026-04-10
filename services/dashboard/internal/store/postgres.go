package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielkuo/project-plant/pkg/telemetry"
	"github.com/danielkuo/project-plant/services/dashboard/internal/api"
)

// PostgresStore implements api.HistoryReader, api.AlertStore, and api.StatsReader.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a store backed by the given connection pool.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// QueryByDevice retrieves historical telemetry for a device within a time range.
func (s *PostgresStore) QueryByDevice(ctx context.Context, deviceID string, tr api.TimeRange, pg api.Pagination) ([]telemetry.TelemetryEvent, error) {
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

// QueryAlerts retrieves alerts matching the given filters.
func (s *PostgresStore) QueryAlerts(ctx context.Context, filters api.AlertFilters) ([]api.Alert, error) {
	query := `SELECT id, device_id, rule_name, severity, triggered_at, resolved_at, reading
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

	var alerts []api.Alert
	for rows.Next() {
		var a api.Alert
		var resolvedAt *time.Time
		var readingJSON []byte
		if err := rows.Scan(&a.AlertID, &a.DeviceID, &a.RuleName, &a.Severity, &a.TriggeredAt, &resolvedAt, &readingJSON); err != nil {
			return nil, err
		}
		a.ResolvedAt = resolvedAt
		if err := json.Unmarshal(readingJSON, &a.Reading); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

// ResolveAlert marks an alert as resolved.
func (s *PostgresStore) ResolveAlert(ctx context.Context, alertID string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE alerts SET resolved_at = NOW() WHERE id = $1 AND resolved_at IS NULL`,
		alertID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("alert not found or already resolved")
	}
	return nil
}

// GetStats returns aggregate fleet statistics.
func (s *PostgresStore) GetStats(ctx context.Context) (api.DeviceStats, error) {
	var stats api.DeviceStats

	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM devices`).Scan(&stats.DeviceCount)
	if err != nil {
		return stats, err
	}

	err = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM telemetry_events`).Scan(&stats.TotalEvents)
	if err != nil {
		return stats, err
	}

	err = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM alerts WHERE resolved_at IS NULL`).Scan(&stats.ActiveAlerts)
	return stats, err
}

// Ping checks Postgres connectivity.
func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}
