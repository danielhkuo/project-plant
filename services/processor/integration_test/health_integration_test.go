//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcPostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"

	"github.com/danielkuo/project-plant/pkg/health"
)

// TestHealthEndpoint_Comprehensive validates the processor's /health
// composition against real dependencies: all connected -> 200 ok, then a
// stopped Postgres flips the endpoint to 503 degraded with the failing
// dependency reported as disconnected. The checks mirror cmd/processor.
func TestHealthEndpoint_Comprehensive(t *testing.T) {
	ctx := context.Background()

	broker, cleanKafka := setupKafka(t)
	defer cleanKafka()
	rdb, cleanRedis := setupRedis(t)
	defer cleanRedis()

	// Postgres with a container handle we can stop mid-test (the shared
	// setupPostgres helper hides the container).
	pgC, err := tcPostgres.Run(ctx, "postgres:17-alpine",
		tcPostgres.WithDatabase("testdb"),
		tcPostgres.WithUsername("test"),
		tcPostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)
	defer pgC.Terminate(ctx)

	connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	handler := health.Handler(map[string]health.Check{
		"kafka": func(ctx context.Context) error {
			var d kafkago.Dialer
			conn, err := d.DialContext(ctx, "tcp", broker)
			if err != nil {
				return err
			}
			defer conn.Close()
			if deadline, ok := ctx.Deadline(); ok {
				conn.SetDeadline(deadline)
			}
			_, err = conn.Brokers()
			return err
		},
		"postgres": pool.Ping,
		"redis": func(ctx context.Context) error {
			return rdb.Ping(ctx).Err()
		},
	})

	probe := func() (int, map[string]string) {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
		var body map[string]string
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		return rec.Code, body
	}

	// All dependencies up.
	code, body := probe()
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ok", body["status"])
	assert.Equal(t, "connected", body["kafka"])
	assert.Equal(t, "connected", body["postgres"])
	assert.Equal(t, "connected", body["redis"])

	// Stop Postgres -> degraded, with only postgres disconnected.
	require.NoError(t, pgC.Stop(ctx, nil))
	code, body = probe()
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, "degraded", body["status"])
	assert.Equal(t, "disconnected", body["postgres"])
	assert.Equal(t, "connected", body["kafka"])
	assert.Equal(t, "connected", body["redis"])
}
