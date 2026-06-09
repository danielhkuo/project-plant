// Command processor consumes telemetry events from Kafka, evaluates alert
// rules, and persists events/alerts to Postgres + Redis.
//
// It wires together the consumer, rule engine, and stores, then runs a Kafka
// read loop that processes each message and commits its offset only after a
// successful (non-retryable) Process call — giving at-least-once delivery with
// Kafka buffering during downstream outages.
package main

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/services/processor/internal/alert"
	"github.com/danielkuo/project-plant/services/processor/internal/config"
	"github.com/danielkuo/project-plant/services/processor/internal/consumer"
	"github.com/danielkuo/project-plant/services/processor/internal/engine"
	"github.com/danielkuo/project-plant/services/processor/internal/store"
)

func main() {
	logger, _ := zap.NewProduction()
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
	pgStore := store.NewPostgresStore(pool)
	redisStore := store.NewRedisStore(redisClient, cfg.RedisTTL)
	eventStore := store.NewCompositeStore(pgStore, redisStore)
	alertPub := alert.NewRedisAlertPublisher(redisPublisher{redisClient})
	ruleEngine := engine.NewDefaultRuleEngine()

	c := consumer.NewConsumer(ruleEngine, eventStore, alertPub, logger)

	// Kafka consumer group reader. FetchMessage + manual CommitMessages gives
	// us at-least-once semantics: an offset is only committed after Process
	// returns nil, so a crash mid-process re-delivers the message.
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:     cfg.KafkaBrokers,
		Topic:       cfg.KafkaTopic,
		GroupID:     cfg.KafkaGroupID,
		StartOffset: kafkago.FirstOffset,
		ErrorLogger: kafkago.LoggerFunc(func(msg string, args ...interface{}) {
			logger.Warn("kafka reader: " + fmt.Sprintf(msg, args...))
		}),
	})
	defer reader.Close()

	// Cancel the read loop on SIGTERM/SIGINT for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	logger.Info("starting processor",
		zap.Strings("brokers", cfg.KafkaBrokers),
		zap.String("topic", cfg.KafkaTopic),
		zap.String("group", cfg.KafkaGroupID),
	)

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

// redisPublisher adapts *redis.Client to alert.RedisClient, whose Publish
// returns a plain error rather than go-redis's *IntCmd.
type redisPublisher struct {
	client *redis.Client
}

func (r redisPublisher) Publish(ctx context.Context, channel string, message interface{}) error {
	return r.client.Publish(ctx, channel, message).Err()
}
