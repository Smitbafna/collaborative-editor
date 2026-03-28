package websocket

import (
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// Hub manages WebSocket connections grouped by document (room).
type Hub struct {
	RoomManager *RoomManager
}

// NewHub creates and returns a new Hub with an initialized RoomManager.
func NewHub() *Hub {
	return &Hub{
		RoomManager: NewRoomManager(),
	}
}

// HandleWebSocket upgrades an HTTP connection to a WebSocket connection
// and joins the client to the document room identified in the URL path.
// Expected URL format: /ws/documents/{documentID}
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Extract document ID from the URL path: /ws/documents/{documentID}
	documentID := extractDocumentID(r.URL.Path)
	if documentID == "" {
		http.Error(w, "document ID is required", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := NewClient(conn)

	log.Printf("client connected\nclient_id=%s\ndocument_id=%s", client.ID, documentID)

	room := h.RoomManager.JoinRoom(documentID, client)

	log.Printf("client joined room\nclient_id=%s\nroom=%s\nclients=%d", client.ID, documentID, len(room.Clients))

	// Start the writer goroutine to send messages from the Send channel to the WebSocket
	go h.writePump(client)

	// Read messages from the client
	go h.handleClient(client, documentID)
}

// handleClient reads messages from a client and broadcasts them to the room.
func (h *Hub) handleClient(client *Client, documentID string) {
	defer func() {
		room, _ := h.RoomManager.GetRoom(documentID)
		remainingClients := 0
		if room != nil {
			remainingClients = len(room.Clients) - 1 // exclude this client (not yet removed)
		}
		h.RoomManager.LeaveRoom(documentID, client.ID)
		client.Conn.Close()
		close(client.Send)
		log.Printf("client left room\nclient_id=%s\nroom=%s\nremaining_clients=%d", client.ID, documentID, remainingClients)
	}()

	for {
		messageType, message, err := client.Conn.ReadMessage()
		if err != nil {
			log.Printf("client %s disconnected: %v", client.ID, err)
			return
		}
		log.Printf("received message from client %s in document %s: %s", client.ID, documentID, message)

		// Broadcast the message to all other clients in the same room
		h.broadcastToRoom(documentID, client.ID, messageType, message)
	}
}

// broadcastToRoom sends a message to all clients in the room.
// If a client's send buffer is full (slow client), the client is disconnected
// to prevent a single slow client from blocking the entire room.
func (h *Hub) broadcastToRoom(documentID string, senderID string, messageType int, message []byte) {
	room, ok := h.RoomManager.GetRoom(documentID)
	if !ok {
		return
	}

	// Copy the clients map to a slice to avoid concurrent map iteration issues
	// if a slow client gets disconnected during iteration.
	var clients []*Client
	for _, c := range room.Clients {
		clients = append(clients, c)
	}

	log.Printf("broadcast message\nroom=%s\nclients=%d", documentID, len(clients))

	for _, client := range clients {
		select {
		case client.Send <- message:
		default:
			// Client's send buffer is full — this client is too slow to keep up.
			// Disconnect the slow client to prevent it from blocking the room.
			log.Printf("client %s is too slow, disconnecting", client.ID)
			go h.disconnectSlowClient(documentID, client)
		}
	}
}

// disconnectSlowClient disconnects a slow client by closing the WebSocket connection.
// Closing the connection will cause handleClient's ReadMessage to return an error,
// which triggers the deferred cleanup (LeaveRoom, close Send channel, etc.).
func (h *Hub) disconnectSlowClient(documentID string, client *Client) {
	// Send a close message to inform the client
	closeMsg := websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "client too slow")
	client.Conn.WriteMessage(websocket.CloseMessage, closeMsg)
	client.Conn.Close()
}

// writePump reads messages from the client's Send channel and writes them to the WebSocket connection.
func (h *Hub) writePump(client *Client) {
	for message := range client.Send {
		if err := client.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
			log.Printf("client %s write error: %v", client.ID, err)
			return
		}
	}
}

// extractDocumentID extracts the document ID from a URL path.
// Expected format: /ws/documents/{documentID}
func extractDocumentID(path string) string {
	// Trim the prefix and return the remaining part
	prefix := "/ws/documents/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	documentID := strings.TrimPrefix(path, prefix)
	// Remove any trailing slash
	documentID = strings.TrimRight(documentID, "/")
	return documentID
}