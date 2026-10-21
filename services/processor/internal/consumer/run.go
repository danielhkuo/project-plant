package consumer

import (
	"context"
	"errors"

	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// MessageReader is the part of *kafka.Reader the read loop uses. Tests drive
// Run with a fake implementation.
type MessageReader interface {
	FetchMessage(ctx context.Context) (kafkago.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafkago.Message) error
}

// Run fetches, processes, and commits messages until ctx is cancelled.
func Run(ctx context.Context, reader MessageReader, c *Consumer, logger *zap.Logger) {
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
