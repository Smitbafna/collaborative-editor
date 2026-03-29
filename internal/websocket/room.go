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