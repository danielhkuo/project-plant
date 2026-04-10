package ws

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"
	"nhooyr.io/websocket"
)

const (
	writeWait  = 10 * time.Second
	sendBuffer = 64
)

// Client represents a single WebSocket connection.
type Client struct {
	hub     *Hub
	conn    *websocket.Conn
	send    chan Message
	devices map[string]bool // nil = accept all devices
	logger  *zap.Logger
}

// NewClient creates a client with an optional device filter.
func NewClient(hub *Hub, conn *websocket.Conn, deviceFilter []string, logger *zap.Logger) *Client {
	var devices map[string]bool
	if len(deviceFilter) > 0 {
		devices = make(map[string]bool, len(deviceFilter))
		for _, id := range deviceFilter {
			devices[id] = true
		}
	}

	return &Client{
		hub:     hub,
		conn:    conn,
		send:    make(chan Message, sendBuffer),
		devices: devices,
		logger:  logger,
	}
}

// Matches returns true if the message passes the client's device filter.
func (c *Client) Matches(msg Message) bool {
	if c.devices == nil {
		return true // no filter = all devices
	}
	if msg.DeviceID == "" {
		return true // messages without device_id (e.g. welcome) pass all filters
	}
	return c.devices[msg.DeviceID]
}

// WritePump reads from the send channel and writes JSON to the WebSocket.
func (c *Client) WritePump(ctx context.Context) {
	defer c.conn.CloseNow()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, writeWait)
			data, err := json.Marshal(msg)
			if err != nil {
				cancel()
				c.logger.Error("failed to marshal ws message", zap.Error(err))
				continue
			}
			err = c.conn.Write(writeCtx, websocket.MessageText, data)
			cancel()
			if err != nil {
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

// ReadPump reads from the WebSocket to detect disconnection.
func (c *Client) ReadPump(ctx context.Context) {
	defer func() {
		c.hub.unregister <- c
		c.conn.CloseNow()
	}()

	for {
		_, _, err := c.conn.Read(ctx)
		if err != nil {
			return
		}
	}
}
