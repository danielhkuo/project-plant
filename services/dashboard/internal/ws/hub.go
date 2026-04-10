package ws

import (
	"context"
	"sync"
)

// Hub manages WebSocket client connections and broadcasts messages.
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan Message
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan Message, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's main event loop. Blocks until ctx is cancelled.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				if client.Matches(msg) {
					select {
					case client.send <- msg:
					default:
						// Client too slow — drop message to avoid blocking
					}
				}
			}
			h.mu.RUnlock()

		case <-ctx.Done():
			h.mu.Lock()
			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
			}
			h.mu.Unlock()
			return
		}
	}
}

// Broadcast sends a message to the broadcast channel.
func (h *Hub) Broadcast(msg Message) {
	select {
	case h.broadcast <- msg:
	default:
		// Broadcast channel full — drop to avoid blocking producer
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
