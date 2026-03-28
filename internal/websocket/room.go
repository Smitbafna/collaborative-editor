package websocket

import "sync"

// Room represents a collaborative document boundary.
//
// Clients within the same Room can communicate with each other.
// Clients in different Rooms are isolated from one another.
type Room struct {
	ID      string
	Clients map[string]*Client
}

// NewRoom creates and returns a new Room with the given ID
// and an initialized Clients map.
func NewRoom(id string) *Room {
	return &Room{
		ID:      id,
		Clients: make(map[string]*Client),
	}
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

	room.Clients[client.ID] = client
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

	delete(room.Clients, clientID)

	// Delete the room if it is now empty
	if len(room.Clients) == 0 {
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

	if len(room.Clients) > 0 {
		return false
	}

	delete(rm.Rooms, id)
	return true
}