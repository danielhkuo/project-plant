package ws_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"nhooyr.io/websocket"

	"github.com/danielkuo/project-plant/services/dashboard/internal/ws"
)

// setupTestServer creates a hub, starts it, and returns an httptest.Server with
// the WebSocket handler mounted. Cleanup is automatic via t.Cleanup.
func setupTestServer(t *testing.T) (*ws.Hub, *httptest.Server) {
	t.Helper()

	hub := ws.NewHub()
	ctx, cancel := context.WithCancel(context.Background())

	go hub.Run(ctx)

	handler := ws.NewWSHandler(hub, zap.NewNop())
	mux := http.NewServeMux()
	mux.Handle("/api/v1/ws/events", handler)
	srv := httptest.NewServer(mux)

	t.Cleanup(func() {
		cancel()
		srv.Close()
	})

	return hub, srv
}

// dial opens a WebSocket connection to the test server.
func dial(t *testing.T, srv *httptest.Server, path string) *websocket.Conn {
	t.Helper()
	url := "ws" + srv.URL[4:] + path // http:// -> ws://
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, url, nil)
	require.NoError(t, err)
	return conn
}

// readMessage reads a single JSON message from the WebSocket with a timeout.
func readMessage(t *testing.T, conn *websocket.Conn, timeout time.Duration) ws.Message {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, data, err := conn.Read(ctx)
	require.NoError(t, err)

	var msg ws.Message
	require.NoError(t, json.Unmarshal(data, &msg))
	return msg
}

func TestWebSocket_Connect(t *testing.T) {
	_, srv := setupTestServer(t)
	conn := dial(t, srv, "/api/v1/ws/events")
	defer conn.CloseNow()

	msg := readMessage(t, conn, 2*time.Second)
	assert.Equal(t, ws.MessageTypeWelcome, msg.Type)
}

func TestWebSocket_ReceivesNewReading(t *testing.T) {
	hub, srv := setupTestServer(t)
	conn := dial(t, srv, "/api/v1/ws/events")
	defer conn.CloseNow()

	// Consume welcome
	readMessage(t, conn, 2*time.Second)

	// Broadcast a reading
	hub.Broadcast(ws.Message{
		Type:     ws.MessageTypeReading,
		DeviceID: "dev-001",
		Payload:  map[string]interface{}{"temperature": 23.5},
	})

	msg := readMessage(t, conn, 2*time.Second)
	assert.Equal(t, ws.MessageTypeReading, msg.Type)
	assert.Equal(t, "dev-001", msg.DeviceID)
}

func TestWebSocket_ReceivesAlert(t *testing.T) {
	hub, srv := setupTestServer(t)
	conn := dial(t, srv, "/api/v1/ws/events")
	defer conn.CloseNow()

	// Consume welcome
	readMessage(t, conn, 2*time.Second)

	hub.Broadcast(ws.Message{
		Type:     ws.MessageTypeAlert,
		DeviceID: "dev-002",
		Payload:  map[string]interface{}{"severity": "critical"},
	})

	msg := readMessage(t, conn, 2*time.Second)
	assert.Equal(t, ws.MessageTypeAlert, msg.Type)
	assert.Equal(t, "dev-002", msg.DeviceID)
}

func TestWebSocket_FilterByDevice(t *testing.T) {
	hub, srv := setupTestServer(t)
	conn := dial(t, srv, "/api/v1/ws/events?devices=dev-001")
	defer conn.CloseNow()

	// Consume welcome
	readMessage(t, conn, 2*time.Second)

	// Broadcast for dev-002 (should be filtered out)
	hub.Broadcast(ws.Message{
		Type:     ws.MessageTypeReading,
		DeviceID: "dev-002",
		Payload:  map[string]interface{}{"temperature": 50.0},
	})

	// Broadcast for dev-001 (should be received)
	hub.Broadcast(ws.Message{
		Type:     ws.MessageTypeReading,
		DeviceID: "dev-001",
		Payload:  map[string]interface{}{"temperature": 23.5},
	})

	msg := readMessage(t, conn, 2*time.Second)
	assert.Equal(t, "dev-001", msg.DeviceID)

	// Verify no dev-002 message arrives
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, _, err := conn.Read(ctx)
	assert.Error(t, err, "should not receive dev-002 message")
}

func TestWebSocket_Disconnect(t *testing.T) {
	hub, srv := setupTestServer(t)
	conn := dial(t, srv, "/api/v1/ws/events")

	// Consume welcome
	readMessage(t, conn, 2*time.Second)
	assert.Equal(t, 1, hub.ClientCount())

	// Close the connection
	conn.Close(websocket.StatusNormalClosure, "bye")

	// Wait for unregister to propagate
	require.Eventually(t, func() bool {
		return hub.ClientCount() == 0
	}, 2*time.Second, 50*time.Millisecond)
}

func TestWebSocket_MultipleClients(t *testing.T) {
	hub, srv := setupTestServer(t)

	const numClients = 10
	conns := make([]*websocket.Conn, numClients)
	for i := 0; i < numClients; i++ {
		conns[i] = dial(t, srv, "/api/v1/ws/events")
		defer conns[i].CloseNow()
	}

	// Consume all welcome messages
	for _, conn := range conns {
		readMessage(t, conn, 2*time.Second)
	}

	// Wait for all clients to register
	require.Eventually(t, func() bool {
		return hub.ClientCount() == numClients
	}, 2*time.Second, 50*time.Millisecond)

	// Broadcast one message
	hub.Broadcast(ws.Message{
		Type:     ws.MessageTypeReading,
		DeviceID: "dev-001",
		Payload:  map[string]interface{}{"temperature": 25.0},
	})

	// All clients should receive it
	var wg sync.WaitGroup
	wg.Add(numClients)
	for i, conn := range conns {
		go func(idx int, c *websocket.Conn) {
			defer wg.Done()
			msg := readMessage(t, c, 2*time.Second)
			assert.Equal(t, ws.MessageTypeReading, msg.Type)
		}(i, conn)
	}
	wg.Wait()
}
