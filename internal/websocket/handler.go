package websocket

import (
	"encoding/json"
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
	Processor   *OperationProcessor
}

// NewHub creates and returns a new Hub with an initialized RoomManager
// and OperationProcessor.
func NewHub() *Hub {
	rm := NewRoomManager()
	return &Hub{
		RoomManager: rm,
		Processor:   NewOperationProcessor(rm),
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

	log.Printf("client joined room\nclient_id=%s\nroom=%s\nclients=%d", client.ID, documentID, room.ClientCount())

	// Send the current document snapshot to the newly joined client so they
	// know the current document state before receiving any operations.
	snapshot := NewDocumentSnapshot(room.GetContent(), int64(room.GetVersion()))
	snapshotData, err := json.Marshal(snapshot)
	if err != nil {
		log.Printf("failed to marshal document snapshot: %v", err)
	} else {
		select {
		case client.Send <- snapshotData:
		default:
			log.Printf("client %s: send buffer full, dropping document snapshot", client.ID)
		}
	}

	// Start the writer goroutine to send messages from the Send channel to the WebSocket
	go h.writePump(client)

	// Read messages from the client
	go h.handleClient(client, documentID)
}

// handleClient reads messages from a client, processes them through the
// OperationProcessor, and broadcasts the results to the room.
//
// The flow is:
//
//	Read message → Parse JSON → Process via OperationProcessor
//	                                     │
//	                        ┌────────────┤
//	                        ▼            ▼
//	                   Send error    Apply operation
//	                   to client     Broadcast to room
//
// The OperationProcessor handles validation and mutation under the room's
// write lock. Broadcasting happens after the lock is released to avoid
// holding the lock during network writes.
func (h *Hub) handleClient(client *Client, documentID string) {
	defer func() {
		room, _ := h.RoomManager.GetRoom(documentID)
		remainingClients := 0
		if room != nil {
			remainingClients = room.ClientCount() - 1 // exclude this client (not yet removed)
		}
		h.RoomManager.LeaveRoom(documentID, client.ID)
		client.Conn.Close()
		close(client.Send)
		log.Printf("client left room\nclient_id=%s\nroom=%s\nremaining_clients=%d", client.ID, documentID, remainingClients)
	}()

	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			log.Printf("client %s disconnected: %v", client.ID, err)
			return
		}
		log.Printf("received message from client %s in document %s: %s", client.ID, documentID, message)

		// Parse the incoming message - could be an operation or a request for missing operations
		var op Operation
		if err := json.Unmarshal(message, &op); err != nil {
			log.Printf("client %s: failed to parse operation: %v", client.ID, err)
			continue
		}

		// Check if this is a request for missing operations instead of an operation
		if op.Type == "request_missing_operations" {
			h.handleMissingOperationsRequest(client, documentID, op)
			continue
		}

		// Delegate to the OperationProcessor for validation and application
		result := h.Processor.Process(documentID, op)

		if result.Err != nil {
			log.Printf("client %s: operation rejected: %v", client.ID, result.Err)

			// Check if this is a stale operation error and transform it
			if result.Err == ErrStaleOperation {
				// Get the room to fetch and transform against missing operations
				if r, ok := h.RoomManager.GetRoom(documentID); ok {
					// Get all operations from the client's base version + 1 to current version
					clientBaseVersion := op.BaseVersion
					missingOps := r.GetOperationsAfter(int(clientBaseVersion))
					
					// Transform the incoming operation against each missing operation
					transformedOp := op
					for _, entry := range missingOps {
						transformedOp = Transform(transformedOp, entry.Operation)
					}

					// Log the transformation for debugging
					log.Printf("operation transformed\noperation_id=%s\nfrom_version=%d\nto_version=%d\ntransformations=%d",
						op.ID, clientBaseVersion, int64(r.Version), len(missingOps))
					
					// Validate the transformed operation against current content
					currentContent := r.GetContent()
					if err := transformedOp.Validate(len(currentContent)); err != nil {
						// If the transformed operation is invalid, apply it as a no-op
						// We still create a new version to maintain the operation log
						transformedOp = TransformToNoop(transformedOp)
					}
					
					// Apply the transformed operation with a guaranteed new version
					_, newVersion := r.ApplyOperationWithVersion(transformedOp)
					
					// Update client's base version to the new version
					client.BaseVersion = int64(newVersion)
					
					log.Printf("client %s: transformed stale operation from v%d through %d missing ops, created v%d", 
						client.ID, clientBaseVersion, len(missingOps), newVersion)

					// Broadcast the transformed operation to all clients (including the sender)
					// so the client knows its operation was accepted and what version it created
					response, err := json.Marshal(NewOperationMessage(transformedOp, int64(newVersion)))
					if err != nil {
						log.Printf("failed to marshal transformed operation: %v", err)
						continue
					}
					
					// Send to all clients in the room (including sender)
					room, _ := h.RoomManager.GetRoom(documentID)
					if room != nil {
						clients := room.GetClientsSnapshot()
						for _, c := range clients {
							select {
							case c.Send <- response:
							default:
								log.Printf("client %s: send buffer full, dropping transformed operation", c.ID)
							}
						}
					}
					continue
				}
			}

			// For all other errors (or if room not found for stale op), send error response
			errorResponse, err := json.Marshal(NewErrorMessage(result.Err.Error()))
			if err != nil {
				log.Printf("failed to marshal error response: %v", err)
				continue
			}
			select {
			case client.Send <- errorResponse:
			default:
				log.Printf("client %s: send buffer full, dropping error response", client.ID)
			}
			continue
		}

		// Idempotency guard: if the processor detected a duplicate, do not
		// broadcast it again to clients. Without this, network retries could
		// cause clients to see the same applied operation twice.
		if result.IsDuplicate {
			log.Printf("client %s: duplicate operation %q ignored (already processed)", client.ID, op.ID)
			continue
		}

		// Broadcast the operation to all other clients in the room.
		// This happens outside the room's lock so that slow network writes
		// do not block other operations.
		//
		// The operation is wrapped in an OperationMessage envelope so that
		// receiving clients can distinguish operations from other message types.
		response, err := json.Marshal(NewOperationMessage(result.Operation, result.Version))
		if err != nil {
			log.Printf("failed to marshal operation for broadcast: %v", err)
			continue
		}

		h.broadcastToRoom(documentID, client.ID, websocket.TextMessage, response)
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

	// Get a snapshot of clients under the room's read lock to avoid
	// concurrent map iteration issues.
	clients := room.GetClientsSnapshot()

	log.Printf("broadcast message\nroom=%s\nclients=%d", documentID, len(clients))

	for _, c := range clients {
		if c.ID == senderID {
			continue
		}
		select {
		case c.Send <- message:
		default:
			// Client's send buffer is full — this client is too slow to keep up.
			// Disconnect the slow client to prevent it from blocking the room.
			log.Printf("client %s is too slow, disconnecting", c.ID)
			go h.disconnectSlowClient(documentID, c)
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

// handleMissingOperationsRequest handles a client's request for missing
// operations after a version gap.
func (h *Hub) handleMissingOperationsRequest(client *Client, documentID string, op Operation) {
	log.Printf("client %s requests operations after version %d", client.ID, op.BaseVersion)

	// Get the room
	room, ok := h.RoomManager.GetRoom(documentID)
	if !ok {
		log.Printf("room %s not found for missing operations request", documentID)
		errorResponse, _ := json.Marshal(NewErrorMessage("room not found"))
		select {
		case client.Send <- errorResponse:
		default:
			log.Printf("client %s: send buffer full, dropping error response", client.ID)
		}
		return
	}

	// Get operations after the client's version
	missingOps := room.GetOperationsAfter(int(op.BaseVersion))

	// Convert to SyncOperation format
	var syncOps []SyncOperation
	for _, entry := range missingOps {
		syncOps = append(syncOps, SyncOperation{
			Version:   int64(entry.Version),
			Operation: entry.Operation,
		})
	}

	// Send the response
	syncMsg := NewSyncRequiredMessage(int64(room.GetVersion()), syncOps)
	response, err := json.Marshal(syncMsg)
	if err != nil {
		log.Printf("failed to marshal sync_required response: %v", err)
		return
	}

	log.Printf("sending %d missing operations to client %s", len(syncOps), client.ID)
	select {
	case client.Send <- response:
	default:
		log.Printf("client %s: send buffer full, dropping sync_required response", client.ID)
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
