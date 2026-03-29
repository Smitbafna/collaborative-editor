package websocket

import (
	"log"
	"sync"
)

// Room represents a collaborative document boundary.
//
// Clients within the same Room can communicate with each other.
// Clients in different Rooms are isolated from one another.
// The Room holds the shared document Content that all clients collaborate on.
type Room struct {
	mu      sync.RWMutex
	ID      string
	Content string
	Clients map[string]*Client
}

// NewRoom creates and returns a new Room with the given ID,
// an initialized Clients map, and empty Content.
func NewRoom(id string) *Room {
	return &Room{
		ID:      id,
		Content: "",
		Clients: make(map[string]*Client),
	}
}

// Broadcast sends a message to all clients in the room.
// If a client's send buffer is full, the message is dropped for that client.
// The optional senderID parameter can be used to exclude the sender from the broadcast.
func (r *Room) Broadcast(message []byte, senderID string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for id, client := range r.Clients {
		if id == senderID {
			continue
		}
		select {
		case client.Send <- message:
		default:
			log.Printf("room %s: dropping message for slow client %s", r.ID, client.ID)
		}
	}
}

// GetContent returns the current document content of the room.
func (r *Room) GetContent() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Content
}

// SetContent sets the document content of the room.
func (r *Room) SetContent(content string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Content = content
}

// ApplyInsert inserts text at the given position in the document content.
//
// The insertion is performed mathematically as:
//
//	content = content[:position] + text + content[position:]
//
// For example, given content "Hello World", inserting "beautiful " at
// position 6 produces "Hello beautiful World":
//
//	content[:6]  = "Hello "
//	text         = "beautiful "
//	content[6:]  = "World"
//	result       = "Hello " + "beautiful " + "World" = "Hello beautiful World"
//
// Position must be in the range [0, len(content)]. If position is out of
// range, the content is returned unchanged.
//
// This method acquires the room's write lock, making it safe to call
// concurrently with other room operations.
func (r *Room) ApplyInsert(position int, text string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.applyInsert(position, text)
}

// applyInsert is the lock-free core of ApplyInsert.
// It must only be called while holding the room's write lock.
func (r *Room) applyInsert(position int, text string) string {
	if position < 0 || position > len(r.Content) {
		// Invalid position, return current content unchanged
		return r.Content
	}
	r.Content = r.Content[:position] + text + r.Content[position:]
	return r.Content
}

// ApplyDelete deletes length characters starting at the given position in the
// document content.
//
// The deletion is performed mathematically as:
//
//	content = content[:position] + content[position+length:]
//
// For example, given content "Hello beautiful World", deleting 10 characters
// at position 6 produces "Hello World":
//
//	content[:6]      = "Hello "
//	content[6+10:]   = content[16:] = "World"
//	result           = "Hello " + "World" = "Hello World"
//
// If position+length extends past the end of the content, only the characters
// up to the end are deleted.
//
// Position must be in the range [0, len(content)-1] and length must be
// positive. If the position is out of range or length is not positive, the
// content is returned unchanged.
//
// This method acquires the room's write lock, making it safe to call
// concurrently with other room operations.
func (r *Room) ApplyDelete(position int, length int) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.applyDelete(position, length)
}

// applyDelete is the lock-free core of ApplyDelete.
// It must only be called while holding the room's write lock.
func (r *Room) applyDelete(position int, length int) string {
	if position < 0 || position >= len(r.Content) {
		// Invalid position, return current content unchanged
		return r.Content
	}
	if length <= 0 {
		// Non-positive length, return current content unchanged
		return r.Content
	}
	end := position + length
	if end > len(r.Content) {
		end = len(r.Content)
	}
	r.Content = r.Content[:position] + r.Content[end:]
	return r.Content
}

// ApplyOperation applies an operation to the document content under a write lock.
//
// The operation is applied atomically:
//  1. Acquire write lock
//  2. Read current document content
//  3. Apply the operation (insert or delete)
//  4. Update the document content
//  5. Release the lock
//  6. Return the updated content
//
// The caller is responsible for broadcasting the result after the lock is released,
// so that slow network writes do not hold up other operations.
func (r *Room) ApplyOperation(op Operation) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch op.Type {
	case InsertOperation:
		return r.applyInsert(op.Position, op.Text)
	case DeleteOperation:
		return r.applyDelete(op.Position, op.Length)
	}

	return r.Content
}

// ClientCount returns the number of clients in the room.
func (r *Room) ClientCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.Clients)
}

// GetClientsSnapshot returns a copy of the clients slice for safe iteration
// outside the room's lock.
func (r *Room) GetClientsSnapshot() []*Client {
	r.mu.RLock()
	defer r.mu.RUnlock()
	clients := make([]*Client, 0, len(r.Clients))
	for _, c := range r.Clients {
		clients = append(clients, c)
	}
	return clients
}

// RoomManager manages all active rooms.
//
// It provides thread-safe operations for creating, joining,
// and leaving rooms, as well as cleaning up empty rooms.
type RoomManager struct {
	mu    sync.RWMutex
	Rooms map[string]*Room
}

// NewRoomManager creates and returns a new RoomManager.
func NewRoomManager() *RoomManager {
	return &RoomManager{
		Rooms: make(map[string]*Room),
	}
}

// CreateRoom creates a new room with the given ID and adds it to the manager.
// If a room with the same ID already exists, it returns the existing room.
func (rm *RoomManager) CreateRoom(id string) *Room {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if room, ok := rm.Rooms[id]; ok {
		return room
	}

	room := NewRoom(id)
	rm.Rooms[id] = room
	return room
}

// GetRoom retrieves a room by its ID.
// Returns the room and a boolean indicating whether it was found.
func (rm *RoomManager) GetRoom(id string) (*Room, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	room, ok := rm.Rooms[id]
	return room, ok
}

// JoinRoom adds a client to the room with the given ID.
// If the room does not exist, it is created first.
func (rm *RoomManager) JoinRoom(roomID string, client *Client) *Room {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	room, ok := rm.Rooms[roomID]
	if !ok {
		room = NewRoom(roomID)
		rm.Rooms[roomID] = room
	}

	room.mu.Lock()
	room.Clients[client.ID] = client
	room.mu.Unlock()

	return room
}

// LeaveRoom removes a client from the room with the given ID.
// If the room becomes empty after removal, it is deleted.
func (rm *RoomManager) LeaveRoom(roomID string, clientID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	room, ok := rm.Rooms[roomID]
	if !ok {
		return
	}

	room.mu.Lock()
	delete(room.Clients, clientID)
	isEmpty := len(room.Clients) == 0
	room.mu.Unlock()

	if isEmpty {
		delete(rm.Rooms, roomID)
	}
}

// DeleteEmptyRoom removes a room from the manager if it has no clients.
// Returns true if the room was deleted, false otherwise.
func (rm *RoomManager) DeleteEmptyRoom(id string) bool {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	room, ok := rm.Rooms[id]
	if !ok {
		return false
	}

	room.mu.RLock()
	hasClients := len(room.Clients) > 0
	room.mu.RUnlock()

	if hasClients {
		return false
	}

	delete(rm.Rooms, id)
	return true
}
