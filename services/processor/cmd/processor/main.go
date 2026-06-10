// Command processor consumes telemetry events from Kafka, evaluates alert
// rules, and persists events/alerts to Postgres + Redis.
//
// It wires together the consumer, rule engine, and stores, then runs a Kafka
// read loop that processes each message and commits its offset only after a
// successful (non-retryable) Process call — giving at-least-once delivery with
// Kafka buffering during downstream outages.
//
// Because the main loop is a Kafka consumer rather than an HTTP server, an
// admin HTTP server on METRICS_ADDR (default :9091) serves /metrics and a
// dependency-aware /health for monitoring and orchestration.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/pkg/health"
	"github.com/danielkuo/project-plant/pkg/logging"
	"github.com/danielkuo/project-plant/services/processor/internal/alert"
	"github.com/danielkuo/project-plant/services/processor/internal/config"
	"github.com/danielkuo/project-plant/services/processor/internal/consumer"
	"github.com/danielkuo/project-plant/services/processor/internal/engine"
	"github.com/danielkuo/project-plant/services/processor/internal/metrics"
	"github.com/danielkuo/project-plant/services/processor/internal/store"
)

func main() {
	logger := logging.MustNew("processor")
	defer logger.Sync()

	cfg := config.Load()

	// Connect to PostgreSQL (durable storage).
	pool, err := pgxpool.New(context.Background(), cfg.PostgresDSN)
	if err != nil {
		logger.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer pool.Close()

	// Connect to Redis (hot cache + alert pub/sub).
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})
	defer redisClient.Close()

	// Assemble processing dependencies.
	m := metrics.New()
	pgStore := store.NewPostgresStore(pool)
	redisStore := store.NewRedisStore(redisClient, cfg.RedisTTL)
	eventStore := store.NewCompositeStore(pgStore, redisStore)
	alertPub := alert.NewRedisAlertPublisher(redisPublisher{redisClient})
	ruleEngine := engine.NewDefaultRuleEngine()

	c := consumer.NewConsumer(ruleEngine, eventStore, alertPub, logger).WithMetrics(m)

	// Kafka consumer group reader. FetchMessage + manual CommitMessages gives
	// us at-least-once semantics: an offset is only committed after Process
	// returns nil, so a crash mid-process re-delivers the message.
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:     cfg.KafkaBrokers,
		Topic:       cfg.KafkaTopic,
		GroupID:     cfg.KafkaGroupID,
		StartOffset: kafkago.FirstOffset,
		// If the first group join races a freshly started broker's coordinator
		// initialization, the assignment can come back empty and the reader
		// idles; watching partition changes forces a rebalance once topic
		// metadata is actually available.
		WatchPartitionChanges: true,
		ErrorLogger: kafkago.LoggerFunc(func(msg string, args ...interface{}) {
			logger.Warn("kafka reader: " + fmt.Sprintf(msg, args...))
		}),
	})
	defer reader.Close()

	// Cancel the read loop on SIGTERM/SIGINT for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// Admin HTTP server: /metrics + dependency-aware /health. A failure here
	// degrades observability but must not take down the pipeline, so errors
	// are logged and consumption continues.
	adminMux := http.NewServeMux()
	adminMux.Handle("GET /metrics", m.Handler())
	adminMux.Handle("GET /health", health.Handler(map[string]health.Check{
		"kafka":    kafkaCheck(cfg.KafkaBrokers),
		"postgres": pool.Ping,
		"redis": func(ctx context.Context) error {
			return redisClient.Ping(ctx).Err()
		},
	}))
	adminSrv := &http.Server{Addr: cfg.MetricsAddr, Handler: adminMux}
	go func() {
		logger.Info("starting admin server", zap.String("addr", cfg.MetricsAddr))
		if err := adminSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("admin server error", zap.Error(err))
		}
	}()

	// Export consumer lag while running. Stats().Lag is the only lag source
	// for group readers (Reader.Lag returns -1 with a GroupID); it refreshes
	// as messages are read, so the gauge can be stale while the topic is idle.
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.SetConsumerLag(reader.Stats().Lag)
			case <-ctx.Done():
				return
			}
		}
	}()

	logger.Info("starting processor",
		zap.Strings("brokers", cfg.KafkaBrokers),
		zap.String("topic", cfg.KafkaTopic),
		zap.String("group", cfg.KafkaGroupID),
	)

	runLoop(ctx, reader, c, logger)

	// Read loop has exited (signal); drain the admin server synchronously so
	// in-flight scrapes/probes finish before the process exits.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	adminSrv.Shutdown(shutdownCtx)
	logger.Info("processor stopped")
}

// runLoop fetches, processes, and commits messages until ctx is cancelled.
func runLoop(ctx context.Context, reader *kafkago.Reader, c *consumer.Consumer, logger *zap.Logger) {
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				logger.Info("shutting down processor")
				return
			}
			logger.Error("fetch message failed", zap.Error(err))
			continue
		}

		if err := c.Process(ctx, msg.Value); err != nil {
			// Retryable failure persisted past the consumer's retries. Do not
			// commit; the message will be re-fetched so Kafka buffers until
			// the downstream store recovers (zero data loss).
			logger.Error("process failed, not committing offset", zap.Error(err))
			continue
		}

		if err := reader.CommitMessages(ctx, msg); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			logger.Error("commit failed", zap.Error(err))
		}
	}
}

// kafkaCheck reports broker reachability by dialing the first broker, bounded
// by the health check's context deadline.
func kafkaCheck(brokers []string) health.Check {
	return func(ctx context.Context) error {
		var d kafkago.Dialer
		conn, err := d.DialContext(ctx, "tcp", brokers[0])
		if err != nil {
			return err
		}
		if deadline, ok := ctx.Deadline(); ok {
			conn.SetDeadline(deadline)
		}
		defer conn.Close()
		_, err = conn.Brokers()
		return err
	}
}

// redisPublisher adapts *redis.Client to alert.RedisClient, whose Publish
// returns a plain error rather than go-redis's *IntCmd.
type redisPublisher struct {
	client *redis.Client
}

func (r redisPublisher) Publish(ctx context.Context, channel string, message interface{}) error {
	return r.client.Publish(ctx, channel, message).Err()
}
