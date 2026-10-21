package consumer_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/prometheus/client_golang/prometheus/testutil"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/pkg/telemetry"
	"github.com/danielkuo/project-plant/services/processor/internal/consumer"
	"github.com/danielkuo/project-plant/services/processor/internal/engine"
	"github.com/danielkuo/project-plant/services/processor/internal/metrics"
)

// fakeReader behaves like a kafka-go group reader on one partition: each
// FetchMessage returns the next message (a fetched message is never handed out
// again in the same session), an idle partition blocks until ctx ends, and a
// closed reader returns io.EOF. fetchErrs are returned first, in order, the way
// kafka-go surfaces broker and group errors without closing the reader. A
// non-nil commitErr fails every commit.
type fakeReader struct {
	mu          sync.Mutex
	fetchErrs   []error
	msgs        []kafkago.Message
	next        int
	closed      bool
	fetches     int
	commitErr   error
	commitCalls int
	commits     []int64
}

func (r *fakeReader) FetchMessage(ctx context.Context) (kafkago.Message, error) {
	r.mu.Lock()
	r.fetches++
	if len(r.fetchErrs) > 0 {
		err := r.fetchErrs[0]
		r.fetchErrs = r.fetchErrs[1:]
		r.mu.Unlock()
		return kafkago.Message{}, err
	}
	if r.closed {
		r.mu.Unlock()
		return kafkago.Message{}, io.EOF
	}
	if r.next < len(r.msgs) {
		m := r.msgs[r.next]
		r.next++
		r.mu.Unlock()
		return m, nil
	}
	r.mu.Unlock()
	<-ctx.Done()
	return kafkago.Message{}, ctx.Err()
}

// CommitMessages records commits even after ctx has ended, so a test catches a
// commit that the real reader might or might not let through.
func (r *fakeReader) CommitMessages(_ context.Context, msgs ...kafkago.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commitCalls++
	if r.commitErr != nil {
		return r.commitErr
	}
	for _, m := range msgs {
		r.commits = append(r.commits, m.Offset)
	}
	return nil
}

func (r *fakeReader) committed() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]int64(nil), r.commits...)
}

func (r *fakeReader) fetchCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fetches
}

func (r *fakeReader) commitCallCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.commitCalls
}

// fakeStore identifies events by temperature. failNext queues errors to return
// for an event before it succeeds; alwaysFail fails it until recover is called.
type fakeStore struct {
	mu         sync.Mutex
	failNext   map[float64][]error
	alwaysFail map[float64]error
	attempts   map[float64]int
	stored     []float64
	afterWrite func()
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		failNext:   map[float64][]error{},
		alwaysFail: map[float64]error{},
		attempts:   map[float64]int{},
	}
}

func (s *fakeStore) Write(_ context.Context, e telemetry.TelemetryEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempts[e.Temperature]++
	if err := s.alwaysFail[e.Temperature]; err != nil {
		return err
	}
	if errs := s.failNext[e.Temperature]; len(errs) > 0 {
		s.failNext[e.Temperature] = errs[1:]
		return errs[0]
	}
	s.stored = append(s.stored, e.Temperature)
	if s.afterWrite != nil {
		s.afterWrite()
	}
	return nil
}

func (s *fakeStore) SetLatest(context.Context, string, telemetry.TelemetryEvent) error { return nil }
func (s *fakeStore) WriteAlert(context.Context, engine.Alert) error                    { return nil }

func (s *fakeStore) recover(temperature float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.alwaysFail, temperature)
}

func (s *fakeStore) snapshot() (stored []float64, attempts map[float64]int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	attempts = make(map[float64]int, len(s.attempts))
	for k, v := range s.attempts {
		attempts[k] = v
	}
	return append([]float64(nil), s.stored...), attempts
}

type nopPublisher struct{}

func (nopPublisher) Publish(context.Context, engine.Alert) error { return nil }

func newConsumer(st consumer.EventStore) *consumer.Consumer {
	return consumer.NewConsumer(engine.NewDefaultRuleEngine(), st, nopPublisher{}, zap.NewNop())
}

// message builds a reading at the given offset. Temperatures stay in the
// normal band so no alert rules fire.
func message(offset int64, temperature float64) kafkago.Message {
	b, _ := json.Marshal(telemetry.TelemetryEvent{
		DeviceID:     "dev-001",
		Timestamp:    time.Now().UTC(),
		Temperature:  temperature,
		Humidity:     60,
		SoilMoisture: 45,
	})
	return kafkago.Message{Offset: offset, Value: b}
}

// startRun runs consumer.Run in the background. stop cancels it and waits for
// it to return; it is also registered as a cleanup so a failed assertion never
// leaks a running loop into later tests. done closes when Run returns.
func startRun(t *testing.T, r consumer.MessageReader, c *consumer.Consumer) (stop func(), done <-chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		consumer.Run(ctx, r, c, zap.NewNop())
	}()
	stop = func() {
		cancel()
		select {
		case <-finished:
		case <-time.After(2 * time.Second):
			t.Error("Run did not return after its context was cancelled")
		}
	}
	t.Cleanup(stop)
	return stop, finished
}

func lastCommitIs(r *fakeReader, offset int64) func() bool {
	return func() bool {
		c := r.committed()
		return len(c) > 0 && c[len(c)-1] == offset
	}
}

func TestRun_RetriesFailedMessageInPlaceUntilStoreRecovers(t *testing.T) {
	defer consumer.SetRetryBackoff(time.Millisecond, 2*time.Millisecond)()
	st := newFakeStore()
	st.alwaysFail[20] = errors.New("connect: connection refused")
	r := &fakeReader{msgs: []kafkago.Message{message(0, 20), message(1, 21)}}
	m := metrics.New()

	stop, _ := startRun(t, r, newConsumer(st).WithMetrics(m))

	// A long outage: far more attempts than any fixed retry budget.
	require.Eventually(t, func() bool {
		_, attempts := st.snapshot()
		return attempts[20] >= 50
	}, 5*time.Second, time.Millisecond)
	assert.Empty(t, r.committed(), "nothing may be committed while offset 0 is unprocessed")

	st.recover(20)
	require.Eventually(t, lastCommitIs(r, 1), 5*time.Second, time.Millisecond)
	stop()

	stored, _ := st.snapshot()
	// Committing offset 1 tells Kafka everything up to 1 is done, so offset 0
	// must have been stored first, not skipped.
	assert.Equal(t, []float64{20, 21}, stored)
	assert.Equal(t, []int64{0, 1}, r.committed())
	assert.Equal(t, 0.0, testutil.ToFloat64(m.EventsProcessedCounter(consumer.ResultError)), "retries are not errors")
	assert.Equal(t, 2.0, testutil.ToFloat64(m.EventsProcessedCounter(consumer.ResultSuccess)))
}

func TestRun_SkipsMessageThatFailsPermanently(t *testing.T) {
	st := newFakeStore()
	// SQLSTATE class 22 (data exception), e.g. a NUL byte in a text column:
	// retrying the same row can never succeed.
	st.alwaysFail[20] = &pgconn.PgError{Code: "22021", Message: "invalid byte sequence for encoding \"UTF8\": 0x00"}
	r := &fakeReader{msgs: []kafkago.Message{message(0, 20), message(1, 21)}}
	m := metrics.New()

	stop, _ := startRun(t, r, newConsumer(st).WithMetrics(m))
	require.Eventually(t, lastCommitIs(r, 1), 5*time.Second, time.Millisecond)
	stop()

	stored, attempts := st.snapshot()
	assert.Equal(t, []float64{21}, stored)
	assert.Equal(t, 1, attempts[20], "a permanent error is not retried")
	assert.Equal(t, []int64{0, 1}, r.committed(), "the bad message is committed past, not left blocking the partition")
	assert.Equal(t, 1.0, testutil.ToFloat64(m.EventsProcessedCounter(consumer.ResultError)))
	assert.Equal(t, 1.0, testutil.ToFloat64(m.EventsProcessedCounter(consumer.ResultSuccess)))
	assert.Equal(t, 0.0, testutil.ToFloat64(m.ProcessRetriesCounter()), "a permanent error is not a retry")
}

// While Run retries one message, kafka-go's lag stats freeze, so the retry
// counter and the last-commit time are the stall signals.
func TestRun_CountsEachRetriedAttempt(t *testing.T) {
	defer consumer.SetRetryBackoff(time.Millisecond, 2*time.Millisecond)()
	st := newFakeStore()
	st.failNext[20] = []error{
		errors.New("connect: connection refused"),
		errors.New("connect: connection refused"),
		errors.New("connect: connection refused"),
	}
	r := &fakeReader{msgs: []kafkago.Message{message(0, 20), message(1, 21)}}
	m := metrics.New()

	stop, _ := startRun(t, r, newConsumer(st).WithMetrics(m))
	require.Eventually(t, lastCommitIs(r, 1), 5*time.Second, time.Millisecond)
	stop()

	assert.Equal(t, 3.0, testutil.ToFloat64(m.ProcessRetriesCounter()))
}

func TestRun_RecordsLastCommitTime(t *testing.T) {
	r := &fakeReader{msgs: []kafkago.Message{message(0, 20)}}
	m := metrics.New()
	before := time.Now()

	stop, _ := startRun(t, r, newConsumer(newFakeStore()).WithMetrics(m))
	require.Eventually(t, lastCommitIs(r, 0), 5*time.Second, time.Millisecond)
	stop() // Run has returned, so the commit's gauge update is visible.
	after := time.Now()

	got := testutil.ToFloat64(m.LastCommitTimestampGauge())
	assert.GreaterOrEqual(t, got, float64(before.UnixNano())/1e9)
	assert.LessOrEqual(t, got, float64(after.UnixNano())/1e9)
}

func TestRun_DoesNotRecordCommitTimeWhenCommitFails(t *testing.T) {
	r := &fakeReader{
		msgs:      []kafkago.Message{message(0, 20)},
		commitErr: errors.New("kafka: coordinator not available"),
	}
	m := metrics.New()

	stop, _ := startRun(t, r, newConsumer(newFakeStore()).WithMetrics(m))
	require.Eventually(t, func() bool { return r.commitCallCount() >= 1 }, 5*time.Second, time.Millisecond)
	stop()

	assert.Equal(t, 0.0, testutil.ToFloat64(m.LastCommitTimestampGauge()), "a failed commit must not look like progress")
}

func TestIsPermanent(t *testing.T) {
	pg := func(code string) error { return &pgconn.PgError{Code: code} }
	cases := []struct {
		name      string
		err       error
		permanent bool
	}{
		{"22021 invalid byte sequence (NUL in text)", pg("22021"), true},
		{"22008 timestamp out of range", pg("22008"), true},
		{"23502 not-null violation", pg("23502"), true},
		{"23514 check violation", pg("23514"), true},
		{"54000 index row too large (huge device_id)", pg("54000"), true},
		{"wrapped 22021", fmt.Errorf("store.Write: %w", pg("22021")), true},
		{"23505 unique violation (a lagging id sequence; retry draws a new id)", pg("23505"), false},
		{"08006 connection failure", pg("08006"), false},
		{"40001 serialization failure", pg("40001"), false},
		{"57P01 admin shutdown", pg("57P01"), false},
		{"plain network error", errors.New("connect: connection refused"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.permanent, consumer.IsPermanent(tc.err))
		})
	}
}

func TestRun_DoesNotCommitWhenStoppedDuringRetries(t *testing.T) {
	st := newFakeStore()
	st.alwaysFail[20] = errors.New("connect: connection refused")
	r := &fakeReader{msgs: []kafkago.Message{message(0, 20)}}

	stop, _ := startRun(t, r, newConsumer(st))
	require.Eventually(t, func() bool {
		_, attempts := st.snapshot()
		return attempts[20] >= 1
	}, 5*time.Second, time.Millisecond)
	stop()

	assert.Empty(t, r.committed(), "an unprocessed message must be redelivered after restart")
}

func TestRun_DoesNotCommitWhenStoppedRightAfterProcessing(t *testing.T) {
	// Shutdown lands after Process succeeds but before the commit. Process
	// may have cut later steps short (alerts), so the message must be
	// redelivered, not committed with a cancelled context.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	st := newFakeStore()
	st.afterWrite = cancel
	r := &fakeReader{msgs: []kafkago.Message{message(0, 20)}}

	done := make(chan struct{})
	go func() {
		defer close(done)
		consumer.Run(ctx, r, newConsumer(st), zap.NewNop())
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after its context was cancelled")
	}

	stored, _ := st.snapshot()
	assert.Equal(t, []float64{20}, stored)
	assert.Empty(t, r.committed())
}

func TestRun_KeepsRunningAfterFetchErrors(t *testing.T) {
	defer consumer.SetRetryBackoff(time.Millisecond, 2*time.Millisecond)()
	// kafka-go returns a bare io.EOF (not only on Close) when the group join
	// fails, e.g. the broker drops the connection mid-JoinGroup. Exiting on it
	// would stop the processor for good.
	r := &fakeReader{
		fetchErrs: []error{io.EOF, errors.New("kafka: broker not available")},
		msgs:      []kafkago.Message{message(0, 20)},
	}

	stop, _ := startRun(t, r, newConsumer(newFakeStore()))
	require.Eventually(t, lastCommitIs(r, 0), 5*time.Second, time.Millisecond)
	stop()
}

func TestRun_BacksOffWhileFetchKeepsFailing(t *testing.T) {
	// A closed reader returns io.EOF on every call. Run must not spin on it;
	// it waits between attempts and stops when its context is cancelled.
	r := &fakeReader{closed: true}

	stop, done := startRun(t, r, newConsumer(newFakeStore()))
	time.Sleep(350 * time.Millisecond)

	select {
	case <-done:
		t.Fatal("Run exited on a fetch error instead of retrying")
	default:
	}
	// 100 ms, then 200 ms between attempts: about 3 fetches in 350 ms.
	assert.LessOrEqual(t, r.fetchCount(), 5, "fetch errors must be retried with backoff, not in a hot loop")
	stop()
}

func TestRun_ReturnsWhenContextDeadlinePasses(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		consumer.Run(ctx, &fakeReader{}, newConsumer(newFakeStore()), zap.NewNop())
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run kept looping after its context deadline passed")
	}
}
