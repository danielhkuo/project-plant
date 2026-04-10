package ws

import (
	"net/http"
	"strings"

	"go.uber.org/zap"
	"nhooyr.io/websocket"
)

// WSHandler upgrades HTTP connections to WebSocket and registers with the Hub.
type WSHandler struct {
	hub    *Hub
	logger *zap.Logger
}

// NewWSHandler creates a WebSocket handler.
func NewWSHandler(hub *Hub, logger *zap.Logger) *WSHandler {
	return &WSHandler{hub: hub, logger: logger}
}

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		h.logger.Error("websocket accept failed", zap.Error(err))
		return
	}

	// Parse optional device filter from query params
	var deviceFilter []string
	if devicesParam := r.URL.Query().Get("devices"); devicesParam != "" {
		for _, d := range strings.Split(devicesParam, ",") {
			if trimmed := strings.TrimSpace(d); trimmed != "" {
				deviceFilter = append(deviceFilter, trimmed)
			}
		}
	}

	client := NewClient(h.hub, conn, deviceFilter, h.logger)
	h.hub.register <- client

	// Send welcome message
	client.send <- Message{
		Type:    MessageTypeWelcome,
		Payload: map[string]string{"message": "connected"},
	}

	ctx := r.Context()
	go client.WritePump(ctx)
	client.ReadPump(ctx)
}
