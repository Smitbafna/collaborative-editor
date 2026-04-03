package websocket

import (
	"github.com/gorilla/websocket"
	"github.com/google/uuid"
)

// Client represents a single WebSocket connection.
//
// It has a unique ID, the underlying WebSocket connection,
// and a buffered Send channel used to serialize outgoing writes.
// Instead of multiple goroutines writing directly to the connection,
// all messages are sent through the Send channel and written by a
// single dedicated writer goroutine, avoiding concurrent writes.
type Client struct {
	ID   string
	Conn *websocket.Conn
	Send chan []byte

	// BaseVersion tracks the document version this client has.
	// It is updated when the client receives operations or snapshots.
	BaseVersion int64
}

// NewClient creates and returns a new Client with a unique ID
// and a buffered Send channel.
func NewClient(conn *websocket.Conn) *Client {
	return &Client{
		ID:   uuid.NewString(),
		Conn: conn,
		Send: make(chan []byte, 256),
	}
}