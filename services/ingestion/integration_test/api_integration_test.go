//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/services/ingestion/internal/api"
	"github.com/danielkuo/project-plant/services/ingestion/internal/auth"
	kafkapkg "github.com/danielkuo/project-plant/services/ingestion/internal/kafka"
)

const testAPIKey = "test-key-001"

// startTestServer wires a real Kafka producer + auth into the API router and
// serves it over a local listener. Each test should use its own topic.
func startTestServer(t *testing.T, broker, topic string) string {
	t.Helper()
	createTopic(t, broker, topic)

	producer := kafkapkg.NewKafkaProducer(kafkapkg.ProducerConfig{
		Brokers:      []string{broker},
		Topic:        topic,
		BatchSize:    1,
		BatchTimeout: time.Millisecond,
		MaxRetries:   3,
	})
	authn := auth.NewStaticKeyAuthenticator(map[string]auth.DeviceIdentity{
		testAPIKey: {DeviceID: "dev-001"},
	})
	router := api.NewRouter(producer, auth.Middleware(authn), nil, zap.NewNop())

	srv := newLocalServer(t, router)
	t.Cleanup(func() { producer.Close() })
	return srv
}

// newLocalServer starts handler on a random localhost port and returns its base
// URL, registering shutdown via t.Cleanup.
func newLocalServer(t *testing.T, handler http.Handler) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &http.Server{Handler: handler}
	go srv.Serve(ln)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	})
	return "http://" + ln.Addr().String()
}

func postTelemetry(t *testing.T, url, apiKey string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url+"/api/v1/telemetry", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func eventJSON(t *testing.T, deviceID string) []byte {
	t.Helper()
	data, err := json.Marshal(testEvent(deviceID))
	require.NoError(t, err)
	return data
}

func TestAPI_EndToEnd_ValidRequest(t *testing.T) {
	broker, cleanup := setupKafka(t)
	defer cleanup()
	topic := "api-valid"
	url := startTestServer(t, broker, topic)

	resp := postTelemetry(t, url, testAPIKey, eventJSON(t, "dev-001"))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	msgs := readMessages(t, broker, topic, 1, 2*time.Second)
	require.Len(t, msgs, 1)
	assert.Equal(t, "dev-001", string(msgs[0].Key))
}

func TestAPI_EndToEnd_AuthRejection(t *testing.T) {
	broker, cleanup := setupKafka(t)
	defer cleanup()
	topic := "api-auth"
	url := startTestServer(t, broker, topic)

	resp := postTelemetry(t, url, "wrong-key", eventJSON(t, "dev-001"))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	msgs := readMessages(t, broker, topic, 1, 1*time.Second)
	assert.Empty(t, msgs, "rejected request must not publish to Kafka")
}

func TestAPI_EndToEnd_ValidationRejection(t *testing.T) {
	broker, cleanup := setupKafka(t)
	defer cleanup()
	topic := "api-validation"
	url := startTestServer(t, broker, topic)

	// Empty device_id fails validation.
	bad := []byte(`{"device_id":"","timestamp":"2026-04-09T12:00:00Z","temperature":23.5,"humidity":62.3,"soil_moisture":45.1}`)
	resp := postTelemetry(t, url, testAPIKey, bad)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	msgs := readMessages(t, broker, topic, 1, 1*time.Second)
	assert.Empty(t, msgs, "invalid request must not publish to Kafka")
}

func TestAPI_EndToEnd_100Concurrent(t *testing.T) {
	broker, cleanup := setupKafka(t)
	defer cleanup()
	topic := "api-concurrent"
	url := startTestServer(t, broker, topic)

	const n = 100
	var wg sync.WaitGroup
	var okCount int64
	var mu sync.Mutex
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp := postTelemetry(t, url, testAPIKey, eventJSON(t, fmt.Sprintf("dev-%03d", i)))
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusAccepted {
				mu.Lock()
				okCount++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	assert.Equal(t, int64(n), okCount, "all concurrent requests should be accepted")
	msgs := readMessages(t, broker, topic, n, 15*time.Second)
	assert.Len(t, msgs, n, "all events should reach Kafka")
}

func TestAPI_Healthcheck(t *testing.T) {
	broker, cleanup := setupKafka(t)
	defer cleanup()
	url := startTestServer(t, broker, "api-health")

	// No API key — /health must be public.
	resp, err := http.Get(url + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"status":"ok","kafka":"connected"}`, string(body))
}

func TestAPI_Healthcheck_KafkaDown(t *testing.T) {
	// Producer points at a dead broker; no real Kafka needed.
	producer := kafkapkg.NewKafkaProducer(kafkapkg.ProducerConfig{
		Brokers: []string{"127.0.0.1:1"},
		Topic:   "telemetry.events",
	})
	t.Cleanup(func() { producer.Close() })
	authn := auth.NewStaticKeyAuthenticator(map[string]auth.DeviceIdentity{testAPIKey: {DeviceID: "dev-001"}})
	url := newLocalServer(t, api.NewRouter(producer, auth.Middleware(authn), nil, zap.NewNop()))

	resp, err := http.Get(url + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"status":"degraded","kafka":"disconnected"}`, string(body))
}

// TestAPI_GracefulShutdown validates the shutdown semantics used by
// cmd/server/main.go: in-flight requests complete and ListenAndServe returns
// ErrServerClosed. We use a slow handler and Shutdown directly rather than
// raising SIGTERM into the test process.
func TestAPI_GracefulShutdown(t *testing.T) {
	started := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("done"))
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &http.Server{Handler: mux}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	respCh := make(chan *http.Response, 1)
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String() + "/slow")
		if err == nil {
			respCh <- resp
		}
	}()

	<-started // request is now in-flight
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, srv.Shutdown(ctx))

	select {
	case resp := <-respCh:
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, "in-flight request should complete")
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight request did not complete during graceful shutdown")
	}

	assert.ErrorIs(t, <-serveErr, http.ErrServerClosed)
}
