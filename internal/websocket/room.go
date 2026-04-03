package websocket

import (
	"log"
	"sync"
)

// VersionedOperation pairs a document operation with the version number
// that was assigned when the operation was applied.
//
// This is the building block of the Room's operation history. Each time
// an operation successfully modifies the document content, a
// VersionedOperation is appended to the room's History slice, creating
// an immutable, ordered log of every change that produced the current
// document state.
type VersionedOperation struct {
	// Version is the document version assigned when this operation was
	// applied. Versions start at 1 for the first operation and increment
	// by 1 for each subsequent operation.
	Version int

	// Operation is the document operation that was applied.
	Operation Operation
}

// Room represents a collaborative document boundary.
//
// Clients within the same Room can communicate with each other.
// Clients in different Rooms are isolated from one another.
// The Room holds the shared document Content that all clients collaborate on.
type Room struct {
	mu                  sync.RWMutex
	ID                  string
	Content             string
	Version             int
	History             []VersionedOperation
	Clients             map[string]*Client
	ProcessedOperations map[string]bool
}

// NewRoom creates and returns a new Room with the given ID,
// an initialized Clients map, and empty Content.
func NewRoom(id string) *Room {
	return &Room{
		ID:                  id,
		Content:             "",
		Version:             0,
		History:             make([]VersionedOperation, 0),
		Clients:             make(map[string]*Client),
		ProcessedOperations: make(map[string]bool),
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

// GetVersion returns the current version of the room.
// The version represents the number of successfully applied operations.
func (r *Room) GetVersion() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Version
}

// GetHistory returns a copy of the room's operation history.
//
// The history is an ordered slice of VersionedOperation entries, one per
// operation that has successfully modified the document. Each entry pairs
// the operation with the version number it was assigned.
//
// The returned slice is a copy, so callers can iterate over it safely
// without holding the room's lock.
func (r *Room) GetHistory() []VersionedOperation {
	r.mu.RLock()
	defer r.mu.RUnlock()
	history := make([]VersionedOperation, len(r.History))
	copy(history, r.History)
	return history
}

// GetHistoryByVersion returns the VersionedOperation stored at the given
// version number, if it exists.
//
// Each successfully applied operation is stored in the room's history
// paired with the version it created (not the base_version it was based on).
// This method allows callers to look up the operation that produced a
// specific document version.
//
// For example, if the current version is 6 and an operation with
// base_version 5 was applied, the history entry will have Version 6
// (the version it created), and GetHistoryByVersion(6) will return it.
// GetHistoryByVersion(5) will return false because no operation was
// *created* at version 5 (version 5 was the base, not the result).
//
// Returns the VersionedOperation and true if found, or a zero-valued
// VersionedOperation and false if no entry exists for the given version.
func (r *Room) GetHistoryByVersion(version int) (VersionedOperation, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, entry := range r.History {
		if entry.Version == version {
			return entry, true
		}
	}
	return VersionedOperation{}, false
}

// GetOperationsAfter returns all operations that were applied after the
// given version number.
//
// This is useful for catching up on changes that a client missed. For example,
// if the current version is 10 and a client asks for operations after version 7,
// this method returns the operations that created versions 8, 9, and 10.
//
// The returned slice is a copy, so callers can iterate over it safely
// without holding the room's lock.
//
// If the version is 0 or negative, all operations in the history are returned.
// If the version is greater than or equal to the current version, an empty
// slice is returned.
func (r *Room) GetOperationsAfter(version int) []VersionedOperation {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// If version is 0 or negative, return all operations
	if version <= 0 {
		history := make([]VersionedOperation, len(r.History))
		copy(history, r.History)
		return history
	}

	// If version is greater than or equal to current version, return empty slice
	if version >= r.Version {
		return []VersionedOperation{}
	}

	// Find the starting index - since history entries have sequential version
	// numbers starting from 1, the entry for version V is at index V-1
	startIndex := version

	// Bounds check
	if startIndex >= len(r.History) {
		return []VersionedOperation{}
	}

	// Return a copy of the slice from startIndex onwards
	result := make([]VersionedOperation, len(r.History)-startIndex)
	copy(result, r.History[startIndex:])
	return result
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

// IsOperationProcessed checks whether an operation with the given ID has
// already been processed by this room. Returns true if the operation ID
// exists in the processed set.
//
// This method must be called while holding the room's read or write lock.
func (r *Room) IsOperationProcessed(opID string) bool {
	return r.ProcessedOperations[opID]
}

// MarkOperationProcessed records an operation ID as processed so that
// duplicate deliveries are ignored.
//
// This method must be called while holding the room's write lock.
func (r *Room) MarkOperationProcessed(opID string) {
	r.ProcessedOperations[opID] = true
}

// ProcessOperationResult is the return type for ProcessOperation.
// It provides both the applied content/version and validation outcome.
type ProcessOperationResult struct {
	Content    string
	Version    int
	Err        error
	IsDuplicate bool
}

// ProcessOperation atomically validates and applies an operation to the document
// under a single write lock. The entire sequence is protected:
//
//	Acquire room lock
//	  │
//	  ▼
//	Check base version
//	  │
//	  ▼
//	Validate operation
//	  │
//	  ▼
//	Apply operation
//	  │
//	  ▼
//	Increment version
//	  │
//	  ▼
//	Store history
//	  │
//	  ▼
//	Release room lock
//
// If the operation is a duplicate (already processed), the current content and
// version are returned without modification.
//
// If the operation is invalid, stale, or has an invalid base version, the
// corresponding error is returned without modifying the document.
//
// This method must be called while NOT holding the room lock—it acquires and
// releases the lock itself.
func (r *Room) ProcessOperation(op Operation) ProcessOperationResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Step 1: Idempotency check — skip duplicate operations
	if op.ID != "" && r.ProcessedOperations[op.ID] {
		return ProcessOperationResult{
			Content:     r.Content,
			Version:     r.Version,
			IsDuplicate: true,
		}
	}

	// Step 2: Check base version (always atomic with the apply)
	if op.BaseVersion != 0 && op.BaseVersion != int64(r.Version) {
		if op.BaseVersion < int64(r.Version) {
			return ProcessOperationResult{
				Err: ErrStaleOperation,
			}
		}
		return ProcessOperationResult{
			Err: ErrInvalidBaseVersion,
		}
	}

	// Step 3: Validate the operation against current content
	if err := op.Validate(len(r.Content)); err != nil {
		return ProcessOperationResult{
			Err: err,
		}
	}

	// Step 4: Apply the operation to the document content
	originalContent := r.Content
	switch op.Type {
	case InsertOperation:
		r.applyInsert(op.Position, op.Text)
	case DeleteOperation:
		r.applyDelete(op.Position, op.Length)
	case NoOpOperation:
		// NO-OP: do nothing
	}

	// Step 5: Increment version after the operation has been successfully applied.
	// The version must only increase after a valid operation is applied;
	// invalid operations (which don't change the content) do not increment it.
	if r.Content != originalContent {
		r.Version++
		r.History = append(r.History, VersionedOperation{
			Version:   r.Version,
			Operation: op,
		})
	}

	// Step 6: Store the operation (mark as processed for idempotency)
	if op.ID != "" {
		r.ProcessedOperations[op.ID] = true
	}

	return ProcessOperationResult{
		Content: r.Content,
		Version: r.Version,
	}
}

// ApplyOperation applies an operation to the document content under a write lock.
//
// This method is retained for backward compatibility but does not include
// base-version checking. For new code, use ProcessOperation instead.
func (r *Room) ApplyOperation(op Operation) (string, int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Idempotency check: skip if this operation was already processed
	if op.ID != "" && r.ProcessedOperations[op.ID] {
		return r.Content, r.Version
	}

	originalContent := r.Content

	// Apply the operation to the document content
	switch op.Type {
	case InsertOperation:
		r.applyInsert(op.Position, op.Text)
	case DeleteOperation:
		r.applyDelete(op.Position, op.Length)
	case NoOpOperation:
		// NO-OP: do nothing
	}

	// Increment version after the operation has been successfully applied.
	// The version must only increase after a valid operation is applied;
	// invalid operations (which don't change the content) do not increment it.
	if r.Content != originalContent {
		r.Version++
		r.History = append(r.History, VersionedOperation{
			Version:   r.Version,
			Operation: op,
		})
	}

	// Store the operation (mark as processed for idempotency)
	if op.ID != "" {
		r.ProcessedOperations[op.ID] = true
	}

	return r.Content, r.Version
}

// ApplyOperationWithVersion applies an operation and always creates a new version,
// even if the operation is a no-op (does not change the content).
//
// This is used when applying transformed stale operations, where the client
// needs to see a new version created so it can synchronize its state.
//
// The method applies the operation under a write lock, records it in history,
// increments the version, and marks it as processed.
func (r *Room) ApplyOperationWithVersion(op Operation) (string, int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Apply the operation to the document content
	switch op.Type {
	case InsertOperation:
		r.applyInsert(op.Position, op.Text)
	case DeleteOperation:
		r.applyDelete(op.Position, op.Length)
	case NoOpOperation:
		// NO-OP: do nothing
	}

	// Always increment version and record in history
	r.Version++
	r.History = append(r.History, VersionedOperation{
		Version:   r.Version,
		Operation: op,
	})

	// Mark as processed for idempotency
	if op.ID != "" {
		r.ProcessedOperations[op.ID] = true
	}

	return r.Content, r.Version
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
