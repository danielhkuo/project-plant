package ws

// MessageType discriminates WebSocket message kinds.
type MessageType string

const (
	MessageTypeWelcome MessageType = "welcome"
	MessageTypeReading MessageType = "reading"
	MessageTypeAlert   MessageType = "alert"
)

// Message is the envelope sent over WebSocket connections.
type Message struct {
	Type     MessageType `json:"type"`
	DeviceID string      `json:"device_id,omitempty"`
	Payload  interface{} `json:"payload"`
}
