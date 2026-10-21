package consumer

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// Backoff between attempts: doubles from min up to max. Variables so tests can
// shorten them (export_test.go).
var (
	minRetryBackoff = 100 * time.Millisecond
	maxRetryBackoff = 5 * time.Second
)

// stallLogAfter is how long one message may be retried before Run logs the
// stall at Error level (once per message; each attempt logs a Warn).
const stallLogAfter = time.Minute

// MessageReader is the part of *kafka.Reader the read loop uses. Tests drive
// Run with a fake implementation.
type MessageReader interface {
	FetchMessage(ctx context.Context) (kafkago.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafkago.Message) error
}

// Run fetches, processes, and commits messages until ctx ends. Callers stop
// it by cancelling ctx and close the reader only after Run returns.
//
// A committed offset means "everything up to here is done", and the reader
// never hands a fetched message out again in the same session. So a message
// that fails is retried in place until it succeeds: moving on and committing
// the next message would silently skip it. The exception is a permanent
// failure (bad data no retry can fix), which is logged, counted, and
// committed past so it cannot block the partition forever.
func Run(ctx context.Context, reader MessageReader, c *Consumer, logger *zap.Logger) {
	fetchBackoff := minRetryBackoff
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				logger.Info("shutting down processor")
				return
			}
			// kafka-go also reports group and broker failures here, including
			// a bare io.EOF when a group join fails, without closing the
			// reader. Wait and fetch again rather than exiting or spinning.
			logger.Error("fetch message failed", zap.Error(err), zap.Duration("backoff", fetchBackoff))
			if !sleep(ctx, fetchBackoff) {
				return
			}
			fetchBackoff = min(fetchBackoff*2, maxRetryBackoff)
			continue
		}
		fetchBackoff = minRetryBackoff

		// Never commit once ctx has ended: Process may have cut later steps
		// (alerts) short, and the commit could race the shutdown. The message
		// is redelivered after restart instead.
		if !processWithRetry(ctx, c, msg, logger) || ctx.Err() != nil {
			return
		}

		if err := reader.CommitMessages(ctx, msg); err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Error("commit failed", zap.Error(err))
		}
	}
}

// processWithRetry processes msg until it succeeds or fails permanently and
// reports whether its offset may be committed. It returns false only when ctx
// ends first.
func processWithRetry(ctx context.Context, c *Consumer, msg kafkago.Message, logger *zap.Logger) bool {
	backoff := minRetryBackoff
	start := time.Now()
	stallLogged := false
	for attempt := 1; ; attempt++ {
		err := c.Process(ctx, msg.Value)
		if err == nil {
			if attempt > 1 {
				logger.Info("message processed after retries",
					zap.Int("partition", msg.Partition), zap.Int64("offset", msg.Offset),
					zap.Int("attempts", attempt), zap.Duration("stalled_for", time.Since(start)))
			}
			return true
		}
		if ctx.Err() != nil {
			return false
		}
		if isPermanent(err) {
			logger.Error("skipping message that can never be stored",
				zap.Error(err), zap.Int("partition", msg.Partition), zap.Int64("offset", msg.Offset))
			c.eventProcessed(ResultError)
			return true
		}

		if !stallLogged && time.Since(start) >= stallLogAfter {
			logger.Error("processing stalled: retrying the same message",
				zap.Error(err), zap.Int("partition", msg.Partition), zap.Int64("offset", msg.Offset),
				zap.Int("attempts", attempt), zap.Duration("stalled_for", time.Since(start)))
			stallLogged = true
		}
		logger.Warn("process failed, retrying the same message",
			zap.Error(err), zap.Int("partition", msg.Partition), zap.Int64("offset", msg.Offset),
			zap.Int("attempt", attempt), zap.Duration("backoff", backoff))
		if !sleep(ctx, backoff) {
			return false
		}
		backoff = min(backoff*2, maxRetryBackoff)
	}
}

// sleep waits for d and reports whether it did; it returns false as soon as
// ctx ends.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// isPermanent reports whether retrying err can never succeed, so the message
// should be skipped rather than block the partition:
//   - SQLSTATE class 22, data exception (a NUL byte in text, an out-of-range
//     timestamp);
//   - class 23, integrity constraint violation, except 23505: telemetry
//     inserts use ON CONFLICT on the dedup key, so a unique violation can only
//     come from a lagging id sequence, and a retry draws a new id;
//   - 54000, program limit exceeded (a device_id too long to index).
func isPermanent(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch {
	case strings.HasPrefix(pgErr.Code, "22"):
		return true
	case strings.HasPrefix(pgErr.Code, "23"):
		return pgErr.Code != "23505"
	default:
		return pgErr.Code == "54000"
	}
}
