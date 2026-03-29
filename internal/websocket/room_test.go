package websocket

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// newTestClient creates a Client with a buffered Send channel and a unique ID,
// but without a real WebSocket connection. This is sufficient for room-level tests.
func newTestClient(t *testing.T) *Client {
	t.Helper()
	return &Client{
		ID:   "test-client-" + t.Name(),
		Send: make(chan []byte, 256),
	}
}

// newTestClientWithID creates a Client with a specific ID, useful for tests
// that need to track which client receives which message.
func newTestClientWithID(t *testing.T, id string) *Client {
	t.Helper()
	return &Client{
		ID:   id,
		Send: make(chan []byte, 256),
	}
}

// TestRoomJoin verifies that a client can join a room and the room
// correctly reports 1 client.
//
// Room initially: 0 clients
// Client joins: 1 client
// Expected: len(room.Clients) == 1
func TestRoomJoin(t *testing.T) {
	room := NewRoom("test-room")

	if got := len(room.Clients); got != 0 {
		t.Errorf("expected 0 clients in new room, got %d", got)
	}

	client := newTestClient(t)
	room.Clients[client.ID] = client

	if got := len(room.Clients); got != 1 {
		t.Errorf("expected 1 client after join, got %d", got)
	}
}

// TestRoomMultipleClients verifies that multiple clients can join a room.
//
// Client A joins
// Client B joins
// Client C joins
// Expected: 3 clients
func TestRoomMultipleClients(t *testing.T) {
	room := NewRoom("test-room")

	clientA := newTestClientWithID(t, "A")
	clientB := newTestClientWithID(t, "B")
	clientC := newTestClientWithID(t, "C")

	room.Clients[clientA.ID] = clientA
	room.Clients[clientB.ID] = clientB
	room.Clients[clientC.ID] = clientC

	if got := len(room.Clients); got != 3 {
		t.Errorf("expected 3 clients, got %d", got)
	}
}

// TestRoomLeave verifies that a client can leave a room and the room
// correctly reports the remaining clients.
//
// A joins
// B joins
// A leaves
// Expected: 1 client remains
func TestRoomLeave(t *testing.T) {
	room := NewRoom("test-room")

	clientA := newTestClientWithID(t, "A")
	clientB := newTestClientWithID(t, "B")

	room.Clients[clientA.ID] = clientA
	room.Clients[clientB.ID] = clientB

	if got := len(room.Clients); got != 2 {
		t.Fatalf("expected 2 clients before leave, got %d", got)
	}

	delete(room.Clients, clientA.ID)

	if got := len(room.Clients); got != 1 {
		t.Errorf("expected 1 client after leave, got %d", got)
	}

	// Verify the remaining client is B
	if _, ok := room.Clients[clientB.ID]; !ok {
		t.Error("expected client B to still be in the room")
	}
}

// TestRoomBroadcast verifies that broadcasting a message sends it to all
// clients in the room.
//
// A joins
// B joins
// Broadcast("hello")
// Expected:
//   A receives "hello"
//   B receives "hello"
func TestRoomBroadcast(t *testing.T) {
	room := NewRoom("test-room")

	clientA := newTestClientWithID(t, "A")
	clientB := newTestClientWithID(t, "B")

	room.Clients[clientA.ID] = clientA
	room.Clients[clientB.ID] = clientB

	message := []byte("hello")
	room.Broadcast(message, "")

	// Both clients should receive the message
	select {
	case msg := <-clientA.Send:
		if string(msg) != "hello" {
			t.Errorf("client A expected 'hello', got '%s'", string(msg))
		}
	case <-time.After(time.Second):
		t.Fatal("client A did not receive the broadcast within timeout")
	}

	select {
	case msg := <-clientB.Send:
		if string(msg) != "hello" {
			t.Errorf("client B expected 'hello', got '%s'", string(msg))
		}
	case <-time.After(time.Second):
		t.Fatal("client B did not receive the broadcast within timeout")
	}
}

// TestRoomBroadcastExcludeSender verifies that broadcasting with a senderID
// excludes the sender from receiving the message.
//
// A joins
// B joins
// Broadcast("hello", sender="A")
// Expected:
//   A receives nothing
//   B receives "hello"
func TestRoomBroadcastExcludeSender(t *testing.T) {
	room := NewRoom("test-room")

	clientA := newTestClientWithID(t, "A")
	clientB := newTestClientWithID(t, "B")

	room.Clients[clientA.ID] = clientA
	room.Clients[clientB.ID] = clientB

	message := []byte("hello")
	room.Broadcast(message, "A")

	// Client A should NOT receive the message (excluded as sender)
	select {
	case msg := <-clientA.Send:
		t.Errorf("client A should not have received the broadcast (excluded as sender), got '%s'", string(msg))
	case <-time.After(100 * time.Millisecond):
		// Expected: no message for A
	}

	// Client B should receive the message
	select {
	case msg := <-clientB.Send:
		if string(msg) != "hello" {
			t.Errorf("client B expected 'hello', got '%s'", string(msg))
		}
	case <-time.After(time.Second):
		t.Fatal("client B did not receive the broadcast within timeout")
	}
}

// TestRoomIsolation verifies that broadcasting to one room does not affect
// clients in a different room.
//
// Room A:
//   ├── Client A
//   └── Client B
//
// Room B:
//   └── Client C
//
// Broadcast to Room A: "hello"
// Expected:
//   Client A ← hello
//   Client B ← hello
//   Client C ← nothing
func TestRoomIsolation(t *testing.T) {
	roomA := NewRoom("room-a")
	roomB := NewRoom("room-b")

	clientA := newTestClientWithID(t, "A")
	clientB := newTestClientWithID(t, "B")
	clientC := newTestClientWithID(t, "C")

	roomA.Clients[clientA.ID] = clientA
	roomA.Clients[clientB.ID] = clientB
	roomB.Clients[clientC.ID] = clientC

	// Broadcast to Room A only
	message := []byte("hello")
	roomA.Broadcast(message, "")

	// Client A should receive the message
	select {
	case msg := <-clientA.Send:
		if string(msg) != "hello" {
			t.Errorf("client A expected 'hello', got '%s'", string(msg))
		}
	case <-time.After(time.Second):
		t.Fatal("client A did not receive the broadcast within timeout")
	}

	// Client B should receive the message
	select {
	case msg := <-clientB.Send:
		if string(msg) != "hello" {
			t.Errorf("client B expected 'hello', got '%s'", string(msg))
		}
	case <-time.After(time.Second):
		t.Fatal("client B did not receive the broadcast within timeout")
	}

	// Client C should NOT receive the message (different room)
	select {
	case msg := <-clientC.Send:
		t.Errorf("client C should not have received a message from room A, got '%s'", string(msg))
	case <-time.After(100 * time.Millisecond):
		// Expected: no message for C
	}
}

// TestRoomManagerJoinAndLeave verifies the RoomManager's JoinRoom and LeaveRoom
// methods work correctly together.
func TestRoomManagerJoinAndLeave(t *testing.T) {
	rm := NewRoomManager()

	clientA := newTestClientWithID(t, "A")
	clientB := newTestClientWithID(t, "B")

	// Join room
	room := rm.JoinRoom("test-room", clientA)
	if got := len(room.Clients); got != 1 {
		t.Errorf("expected 1 client after join, got %d", got)
	}

	// Join second client
	rm.JoinRoom("test-room", clientB)
	if got := len(room.Clients); got != 2 {
		t.Errorf("expected 2 clients after second join, got %d", got)
	}

	// Leave first client
	rm.LeaveRoom("test-room", clientA.ID)
	if got := len(room.Clients); got != 1 {
		t.Errorf("expected 1 client after leave, got %d", got)
	}

	// Verify room still exists (not empty yet)
	if _, ok := rm.GetRoom("test-room"); !ok {
		t.Error("expected room to still exist with 1 client")
	}

	// Leave second client
	rm.LeaveRoom("test-room", clientB.ID)

	// Room should be deleted since it's now empty
	if _, ok := rm.GetRoom("test-room"); ok {
		t.Error("expected room to be deleted after all clients left")
	}
}

// TestRoomManagerCreateRoom verifies that CreateRoom creates a new room
// and returns the existing room if it already exists.
func TestRoomManagerCreateRoom(t *testing.T) {
	rm := NewRoomManager()

	room1 := rm.CreateRoom("test-room")
	if room1 == nil {
		t.Fatal("expected non-nil room from CreateRoom")
	}

	room2 := rm.CreateRoom("test-room")
	if room2 != room1 {
		t.Error("expected CreateRoom to return the same room for an existing ID")
	}
}

// TestRoomManagerDeleteEmptyRoom verifies that DeleteEmptyRoom only deletes
// rooms with no clients.
func TestRoomManagerDeleteEmptyRoom(t *testing.T) {
	rm := NewRoomManager()

	// Create a room with a client
	client := newTestClient(t)
	rm.JoinRoom("test-room", client)

	// Should not delete a non-empty room
	if deleted := rm.DeleteEmptyRoom("test-room"); deleted {
		t.Error("expected DeleteEmptyRoom to return false for non-empty room")
	}

	// Manually remove the client from the room map (without going through LeaveRoom,
	// since LeaveRoom auto-deletes empty rooms).
	rm.mu.Lock()
	room := rm.Rooms["test-room"]
	delete(room.Clients, client.ID)
	rm.mu.Unlock()

	// Now should delete the empty room
	if deleted := rm.DeleteEmptyRoom("test-room"); !deleted {
		t.Error("expected DeleteEmptyRoom to return true for empty room")
	}

	// Room should be gone
	if _, ok := rm.GetRoom("test-room"); ok {
		t.Error("expected room to be deleted")
	}
}

// TestRoomNewRoomHasEmptyContent verifies that a newly created room
// starts with an empty document.
//
// NewRoom("test-room")
// Expected: Content == ""
func TestRoomNewRoomHasEmptyContent(t *testing.T) {
	room := NewRoom("test-room")

	if got := room.GetContent(); got != "" {
		t.Errorf("expected empty content for new room, got '%s'", got)
	}
}

// TestRoomSetAndGetContent verifies that content can be set and retrieved
// from a room.
//
// SetContent("Hello World")
// GetContent()
// Expected: "Hello World"
func TestRoomSetAndGetContent(t *testing.T) {
	room := NewRoom("test-room")

	room.SetContent("Hello World")

	if got := room.GetContent(); got != "Hello World" {
		t.Errorf("expected 'Hello World', got '%s'", got)
	}
}

// TestRoomContentIsolation verifies that content is isolated between rooms.
//
// Room A: SetContent("Hello from A")
// Room B: SetContent("Hello from B")
// Expected:
//   Room A.GetContent() == "Hello from A"
//   Room B.GetContent() == "Hello from B"
func TestRoomContentIsolation(t *testing.T) {
	roomA := NewRoom("room-a")
	roomB := NewRoom("room-b")

	roomA.SetContent("Hello from A")
	roomB.SetContent("Hello from B")

	if got := roomA.GetContent(); got != "Hello from A" {
		t.Errorf("room A expected 'Hello from A', got '%s'", got)
	}

	if got := roomB.GetContent(); got != "Hello from B" {
		t.Errorf("room B expected 'Hello from B', got '%s'", got)
	}
}

// TestRoomContentOverwrite verifies that content can be overwritten.
//
// SetContent("Hello")
// SetContent("World")
// GetContent()
// Expected: "World"
func TestRoomContentOverwrite(t *testing.T) {
	room := NewRoom("test-room")

	room.SetContent("Hello")
	room.SetContent("World")

	if got := room.GetContent(); got != "World" {
		t.Errorf("expected 'World' after overwrite, got '%s'", got)
	}
}

// TestRoomApplyOperationInsert verifies that an insert operation correctly
// modifies the document content.
//
// Initial content: "Hello World"
// Insert "Beautiful " at position 6
// Expected: "Hello Beautiful World"
func TestRoomApplyOperationInsert(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello World")

	op := Operation{
		Type:     InsertOperation,
		Position: 6,
		Text:     "Beautiful ",
	}

	updated := room.ApplyOperation(op)

	expected := "Hello Beautiful World"
	if updated != expected {
		t.Errorf("expected '%s', got '%s'", expected, updated)
	}

	// Verify the room content was actually updated
	if got := room.GetContent(); got != expected {
		t.Errorf("room content expected '%s', got '%s'", expected, got)
	}
}

// TestRoomApplyOperationInsertAtBeginning verifies inserting at position 0.
//
// Initial content: "World"
// Insert "Hello " at position 0
// Expected: "Hello World"
func TestRoomApplyOperationInsertAtBeginning(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("World")

	op := Operation{
		Type:     InsertOperation,
		Position: 0,
		Text:     "Hello ",
	}

	updated := room.ApplyOperation(op)

	expected := "Hello World"
	if updated != expected {
		t.Errorf("expected '%s', got '%s'", expected, updated)
	}
}

// TestRoomApplyOperationInsertAtEnd verifies inserting at the end of the content.
//
// Initial content: "Hello"
// Insert " World" at position 5
// Expected: "Hello World"
func TestRoomApplyOperationInsertAtEnd(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello")

	op := Operation{
		Type:     InsertOperation,
		Position: 5,
		Text:     " World",
	}

	updated := room.ApplyOperation(op)

	expected := "Hello World"
	if updated != expected {
		t.Errorf("expected '%s', got '%s'", expected, updated)
	}
}

// TestRoomApplyOperationInsertInvalidPosition verifies that an insert with
// an invalid position (beyond content length) does not modify the content.
//
// Initial content: "Hello"
// Insert "X" at position 10 (invalid)
// Expected: "Hello" (unchanged)
func TestRoomApplyOperationInsertInvalidPosition(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello")

	op := Operation{
		Type:     InsertOperation,
		Position: 10,
		Text:     "X",
	}

	updated := room.ApplyOperation(op)

	expected := "Hello"
	if updated != expected {
		t.Errorf("expected '%s' (unchanged), got '%s'", expected, updated)
	}
}

// TestRoomApplyOperationInsertNegativePosition verifies that an insert with
// a negative position does not modify the content.
//
// Initial content: "Hello"
// Insert "X" at position -1 (invalid)
// Expected: "Hello" (unchanged)
func TestRoomApplyOperationInsertNegativePosition(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello")

	op := Operation{
		Type:     InsertOperation,
		Position: -1,
		Text:     "X",
	}

	updated := room.ApplyOperation(op)

	expected := "Hello"
	if updated != expected {
		t.Errorf("expected '%s' (unchanged), got '%s'", expected, updated)
	}
}

// TestRoomApplyOperationDelete verifies that a delete operation correctly
// removes characters from the document content.
//
// Initial content: "Hello World"
// Delete 5 characters at position 6 (deletes "World")
// Expected: "Hello "
func TestRoomApplyOperationDelete(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello World")

	op := Operation{
		Type:     DeleteOperation,
		Position: 6,
		Length:   5,
	}

	updated := room.ApplyOperation(op)

	expected := "Hello "
	if updated != expected {
		t.Errorf("expected '%s', got '%s'", expected, updated)
	}
}

// TestRoomApplyOperationDeleteFromBeginning verifies deleting from position 0.
//
// Initial content: "Hello World"
// Delete 6 characters at position 0
// Expected: "World"
func TestRoomApplyOperationDeleteFromBeginning(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello World")

	op := Operation{
		Type:     DeleteOperation,
		Position: 0,
		Length:   6,
	}

	updated := room.ApplyOperation(op)

	expected := "World"
	if updated != expected {
		t.Errorf("expected '%s', got '%s'", expected, updated)
	}
}

// TestRoomApplyOperationDeleteInvalidPosition verifies that a delete with
// an invalid position (beyond content length) does not modify the content.
//
// Initial content: "Hello"
// Delete 1 character at position 10 (invalid)
// Expected: "Hello" (unchanged)
func TestRoomApplyOperationDeleteInvalidPosition(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello")

	op := Operation{
		Type:     DeleteOperation,
		Position: 10,
		Length:   1,
	}

	updated := room.ApplyOperation(op)

	expected := "Hello"
	if updated != expected {
		t.Errorf("expected '%s' (unchanged), got '%s'", expected, updated)
	}
}

// TestRoomApplyOperationDeleteNegativePosition verifies that a delete with
// a negative position does not modify the content.
//
// Initial content: "Hello"
// Delete 1 character at position -1 (invalid)
// Expected: "Hello" (unchanged)
func TestRoomApplyOperationDeleteNegativePosition(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello")

	op := Operation{
		Type:     DeleteOperation,
		Position: -1,
		Length:   1,
	}

	updated := room.ApplyOperation(op)

	expected := "Hello"
	if updated != expected {
		t.Errorf("expected '%s' (unchanged), got '%s'", expected, updated)
	}
}

// TestRoomApplyOperationDeletePastEnd verifies that a delete with length
// extending past the end of the content only deletes up to the end.
//
// Initial content: "Hello"
// Delete 100 characters at position 3 (only 2 characters remain)
// Expected: "Hel"
func TestRoomApplyOperationDeletePastEnd(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello")

	op := Operation{
		Type:     DeleteOperation,
		Position: 3,
		Length:   100,
	}

	updated := room.ApplyOperation(op)

	expected := "Hel"
	if updated != expected {
		t.Errorf("expected '%s', got '%s'", expected, updated)
	}
}

// TestRoomApplyOperationInsertOnEmptyContent verifies inserting into empty content.
//
// Initial content: ""
// Insert "Hello" at position 0
// Expected: "Hello"
func TestRoomApplyOperationInsertOnEmptyContent(t *testing.T) {
	room := NewRoom("test-room")

	op := Operation{
		Type:     InsertOperation,
		Position: 0,
		Text:     "Hello",
	}

	updated := room.ApplyOperation(op)

	expected := "Hello"
	if updated != expected {
		t.Errorf("expected '%s', got '%s'", expected, updated)
	}
}

// TestRoomApplyOperationDeleteOnEmptyContent verifies deleting from empty content.
//
// Initial content: ""
// Delete 1 character at position 0 (invalid — empty content)
// Expected: "" (unchanged)
func TestRoomApplyOperationDeleteOnEmptyContent(t *testing.T) {
	room := NewRoom("test-room")

	op := Operation{
		Type:     DeleteOperation,
		Position: 0,
		Length:   1,
	}

	updated := room.ApplyOperation(op)

	expected := ""
	if updated != expected {
		t.Errorf("expected '%s' (unchanged), got '%s'", expected, updated)
	}
}

// TestRoomApplyOperationMultiple verifies that multiple operations can be
// applied sequentially to build up content.
//
// Insert "Hello" at 0 → "Hello"
// Insert " World" at 5 → "Hello World"
// Delete 6 at 0 → "World"
// Insert "Goodbye " at 0 → "Goodbye World"
func TestRoomApplyOperationMultiple(t *testing.T) {
	room := NewRoom("test-room")

	op1 := Operation{Type: InsertOperation, Position: 0, Text: "Hello"}
	room.ApplyOperation(op1)

	op2 := Operation{Type: InsertOperation, Position: 5, Text: " World"}
	room.ApplyOperation(op2)

	op3 := Operation{Type: DeleteOperation, Position: 0, Length: 6}
	room.ApplyOperation(op3)

	op4 := Operation{Type: InsertOperation, Position: 0, Text: "Goodbye "}
	room.ApplyOperation(op4)

	expected := "Goodbye World"
	if got := room.GetContent(); got != expected {
		t.Errorf("expected '%s', got '%s'", expected, got)
	}
}

// TestRoomApplyOperationConcurrent verifies that ApplyOperation can be called
// concurrently from multiple goroutines without data races.
func TestRoomApplyOperationConcurrent(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("")

	const goroutines = 10
	const operationsPerGoroutine = 50

	done := make(chan bool)

	for range goroutines {
		go func() {
			for range operationsPerGoroutine {
				op := Operation{
					Type:     InsertOperation,
					Position: 0,
					Text:     "a",
				}
				room.ApplyOperation(op)
			}
			done <- true
		}()
	}

	// Wait for all goroutines to finish
	for range goroutines {
		<-done
	}

	// Content should have all the inserted characters
	expectedLen := goroutines * operationsPerGoroutine
	if got := len(room.GetContent()); got != expectedLen {
		t.Errorf("expected content length %d, got %d", expectedLen, got)
	}
}

// TestRoomContentConcurrentAccess verifies that content can be safely
// read and written concurrently without data races.
func TestRoomContentConcurrentAccess(t *testing.T) {
	room := NewRoom("test-room")

	const goroutines = 10
	const iterations = 100

	done := make(chan bool)

	// Concurrent writers
	for range goroutines {
		go func() {
			for range iterations {
				room.SetContent("concurrent content")
			}
			done <- true
		}()
	}

	// Concurrent readers
	for range goroutines {
		go func() {
			for range iterations {
				_ = room.GetContent()
			}
			done <- true
		}()
	}

	// Wait for all goroutines to finish
	for range goroutines * 2 {
		<-done
	}

	// Verify content is still accessible and correct
	_ = room.GetContent()
}

// ---------------------------------------------------------------------------
// Integration tests — test the full WebSocket server pipeline
// ---------------------------------------------------------------------------

// connectClient is a test helper that dials the test WebSocket server at the
// given URL and returns the active *websocket.Conn.
func connectClient(t *testing.T, serverURL, documentID string) *websocket.Conn {
	t.Helper()

	// Convert http:// to ws:// (or https:// to wss://)
	wsURL := "ws://" + serverURL[len("http://"):]
	u := wsURL + "/ws/documents/" + documentID
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatalf("failed to connect to %s: %v", u, err)
	}
	return conn
}

// readMessage is a test helper that reads a single message from the WebSocket
// connection with a timeout. It fails the test if no message arrives in time.
func readMessage(t *testing.T, conn *websocket.Conn, timeout time.Duration) []byte {
	t.Helper()

	conn.SetReadDeadline(time.Now().Add(timeout))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}
	return message
}

// expectNoMessage is a test helper that asserts no message arrives on the
// WebSocket connection within the given duration.
func expectNoMessage(t *testing.T, conn *websocket.Conn, label string, wait time.Duration) {
	t.Helper()

	conn.SetReadDeadline(time.Now().Add(wait))
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Errorf("%s: expected no message but received one", label)
		return
	}
	if !websocket.IsCloseError(err, websocket.CloseNormalClosure) &&
		!websocket.IsUnexpectedCloseError(err) &&
		!strings.Contains(err.Error(), "i/o timeout") &&
		!strings.Contains(err.Error(), "timeout") {
		// A timeout error is expected — any other error is suspicious
		t.Logf("%s: read returned (expected timeout): %v", label, err)
	}
}

// sendOperation is a test helper that serializes an Operation and sends it as JSON.
func sendOperation(t *testing.T, conn *websocket.Conn, op Operation) {
	t.Helper()

	data, err := json.Marshal(op)
	if err != nil {
		t.Fatalf("failed to marshal operation: %v", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("failed to send operation: %v", err)
	}
}

// TestIntegrationWebSocketBroadcastSameRoom verifies the full WebSocket pipeline:
//
//  1. Start a test HTTP server with the WebSocket handler
//  2. Connect Client A to document-123
//  3. Connect Client B to document-123
//  4. Client A sends an insert operation
//  5. Client B receives the operation broadcast as-is and applies it locally
//
// This validates:
//   WebSocket → Connection → Room Manager → Room → ApplyOperation → Broadcast → Other WebSocket
func TestIntegrationWebSocketBroadcastSameRoom(t *testing.T) {
	// Start a test HTTP server
	hub := NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", hub.HandleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Connect Client A to document-123
	connA := connectClient(t, server.URL, "document-123")
	defer connA.Close()

	// Connect Client B to document-123
	connB := connectClient(t, server.URL, "document-123")
	defer connB.Close()

	// Client A sends an insert operation
	op := Operation{
		Type:     InsertOperation,
		Position: 0,
		Text:     "Hello World",
	}
	sendOperation(t, connA, op)

	// Client B should receive the raw operation broadcast
	got := readMessage(t, connB, time.Second)

	var receivedOp Operation
	if err := json.Unmarshal(got, &receivedOp); err != nil {
		t.Fatalf("failed to parse broadcast operation JSON: %v", err)
	}

	if receivedOp.Type != InsertOperation {
		t.Errorf("expected operation type 'insert', got '%v'", receivedOp.Type)
	}
	if receivedOp.Position != 0 {
		t.Errorf("expected position 0, got %d", receivedOp.Position)
	}
	if receivedOp.Text != "Hello World" {
		t.Errorf("expected text 'Hello World', got '%s'", receivedOp.Text)
	}
}

// TestIntegrationConcurrentConnections verifies that 50 clients can connect
// concurrently to the same room without data races or server crashes.
//
// 50 clients ── connect concurrently to the same room
//
// Expected:
//   - All connections succeed
//   - No data races
//   - No server crash
func TestIntegrationConcurrentConnections(t *testing.T) {
	// Start a test HTTP server
	hub := NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", hub.HandleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	const numClients = 50

	type result struct {
		index int
		conn  *websocket.Conn
		err   error
	}

	results := make(chan result, numClients)

	// Launch 50 clients concurrently
	for i := range numClients {
		go func(idx int) {
			wsURL := "ws://" + server.URL[len("http://"):]
			u := wsURL + "/ws/documents/concurrent-room"
			conn, _, err := websocket.DefaultDialer.Dial(u, nil)
			results <- result{idx, conn, err}
		}(i)
	}

	// Collect all results
	connections := make([]*websocket.Conn, 0, numClients)
	successCount := 0
	for range numClients {
		r := <-results
		if r.err != nil {
			t.Errorf("client %d failed to connect: %v", r.index, r.err)
			continue
		}
		successCount++
		connections = append(connections, r.conn)
	}

	// Verify all connections succeeded
	if successCount != numClients {
		t.Errorf("expected %d successful connections, got %d", numClients, successCount)
	}

	// Wait for all handlers to finish registering clients in the room.
	// Dial() returns after the HTTP upgrade, but the handler may not have
	// completed JoinRoom yet, so we poll with a timeout.
	waitForClients := func(expected int) bool {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			room, ok := hub.RoomManager.GetRoom("concurrent-room")
			if ok && room.ClientCount() == expected {
				return true
			}
			time.Sleep(10 * time.Millisecond)
		}
		return false
	}

	if !waitForClients(numClients) {
		room, ok := hub.RoomManager.GetRoom("concurrent-room")
		if !ok {
			t.Fatalf("expected room 'concurrent-room' to exist after waiting")
		}
		t.Errorf("expected %d clients in room after waiting, got %d", numClients, room.ClientCount())
	}

	// Test that broadcasting works with all clients connected
	// Send an operation from client 0, all others should receive the raw operation
	op := Operation{
		Type:     InsertOperation,
		Position: 0,
		Text:     "concurrent-test",
	}
	sendOperation(t, connections[0], op)

	// All other clients should receive the raw operation broadcast
	for i := 1; i < len(connections); i++ {
		connections[i].SetReadDeadline(time.Now().Add(5 * time.Second))
		_, msg, err := connections[i].ReadMessage()
		if err != nil {
			t.Errorf("client %d failed to receive message: %v", i, err)
			continue
		}

		var receivedOp Operation
		if err := json.Unmarshal(msg, &receivedOp); err != nil {
			t.Errorf("client %d: failed to parse operation JSON: %v", i, err)
			continue
		}

		if receivedOp.Type != InsertOperation {
			t.Errorf("client %d: expected operation type 'insert', got '%v'", i, receivedOp.Type)
		}
		if receivedOp.Position != 0 {
			t.Errorf("client %d: expected position 0, got %d", i, receivedOp.Position)
		}
		if receivedOp.Text != "concurrent-test" {
			t.Errorf("client %d: expected text 'concurrent-test', got '%s'", i, receivedOp.Text)
		}
	}

	// Clean up all connections
	for _, conn := range connections {
		conn.Close()
	}
}

// TestIntegrationWebSocketRoomIsolation verifies that clients in different
// rooms do not receive each other's messages:
//
//  1. Start a test HTTP server
//  2. Connect Client A to document-123
//  3. Connect Client B to document-123
//  4. Connect Client C to document-456
//  5. Client A sends "world"
//  6. Client B receives "world"
//  7. Client C must NOT receive "world"
//
// This validates room isolation through the full pipeline.
func TestIntegrationWebSocketRoomIsolation(t *testing.T) {
	// Start a test HTTP server
	hub := NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", hub.HandleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Connect Client A to document-123
	connA := connectClient(t, server.URL, "document-123")
	defer connA.Close()

	// Connect Client B to document-123
	connB := connectClient(t, server.URL, "document-123")
	defer connB.Close()

	// Connect Client C to document-456 (different room)
	connC := connectClient(t, server.URL, "document-456")
	defer connC.Close()

	// Client A sends an insert operation
	op := Operation{
		Type:     InsertOperation,
		Position: 0,
		Text:     "world",
	}
	sendOperation(t, connA, op)

	// Client B (same room) should receive the raw operation broadcast
	got := readMessage(t, connB, time.Second)

	var receivedOp Operation
	if err := json.Unmarshal(got, &receivedOp); err != nil {
		t.Fatalf("failed to parse broadcast operation JSON: %v", err)
	}

	if receivedOp.Type != InsertOperation {
		t.Errorf("expected operation type 'insert', got '%v'", receivedOp.Type)
	}
	if receivedOp.Position != 0 {
		t.Errorf("expected position 0, got %d", receivedOp.Position)
	}
	if receivedOp.Text != "world" {
		t.Errorf("expected text 'world', got '%s'", receivedOp.Text)
	}

	// Client C (different room) must NOT receive "world"
	expectNoMessage(t, connC, "client C", 200*time.Millisecond)
}

// ---------------------------------------------------------------------------
// Validation integration tests — verify the handler rejects invalid operations
// ---------------------------------------------------------------------------

// TestIntegrationRejectInvalidPosition verifies that the server rejects an
// INSERT operation with a position out of bounds.
//
// Document: "Hello" (length 5)
// Operation: INSERT at position 100
// Expected: error response with "position out of bounds", no broadcast
func TestIntegrationRejectInvalidPosition(t *testing.T) {
	hub := NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", hub.HandleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	connA := connectClient(t, server.URL, "doc-validation")
	defer connA.Close()

	connB := connectClient(t, server.URL, "doc-validation")
	defer connB.Close()

	// Seed the document with "Hello"
	seedOp := Operation{
		Type:     InsertOperation,
		Position: 0,
		Text:     "Hello",
	}
	sendOperation(t, connA, seedOp)

	// A is the sender so it gets no broadcast.
	// B receives the raw operation broadcast confirming the seed was applied.
	seedResp := readMessage(t, connB, time.Second)
	var seedOpReceived Operation
	if err := json.Unmarshal(seedResp, &seedOpReceived); err != nil {
		t.Fatalf("failed to parse seed broadcast on B: %v", err)
	}
	if seedOpReceived.Type != InsertOperation {
		t.Fatalf("expected insert operation for seed on B, got '%v'", seedOpReceived.Type)
	}
	if seedOpReceived.Position != 0 {
		t.Fatalf("expected position 0 for seed, got %d", seedOpReceived.Position)
	}
	if seedOpReceived.Text != "Hello" {
		t.Fatalf("expected text 'Hello' for seed, got '%s'", seedOpReceived.Text)
	}

	// Now send an invalid insert at position 100
	invalidOp := Operation{
		Type:     InsertOperation,
		Position: 100,
		Text:     "X",
	}
	sendOperation(t, connA, invalidOp)

	// Client A should receive an error response (the only message A gets)
	connA.SetReadDeadline(time.Now().Add(time.Second))
	_, msgA, err := connA.ReadMessage()
	if err != nil {
		t.Fatalf("client A did not receive an error response: %v", err)
	}

	var errResp map[string]interface{}
	if err := json.Unmarshal(msgA, &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	if errResp["type"] != "error" {
		t.Errorf("expected error response type, got '%v'", errResp["type"])
	}
	errMsg, _ := errResp["error"].(string)
	if !strings.Contains(errMsg, "position out of bounds") {
		t.Errorf("expected 'position out of bounds' error, got '%v'", errResp["error"])
	}

	// Client B should NOT receive any broadcast for the invalid operation
	expectNoMessage(t, connB, "client B (invalid position)", 200*time.Millisecond)
}

// TestIntegrationRejectEmptyText verifies that the server rejects an INSERT
// operation with empty text.
//
// Document: "Hello"
// Operation: INSERT "" at position 2
// Expected: error response, no broadcast
func TestIntegrationRejectEmptyText(t *testing.T) {
	hub := NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", hub.HandleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	connA := connectClient(t, server.URL, "doc-empty-text")
	defer connA.Close()

	connB := connectClient(t, server.URL, "doc-empty-text")
	defer connB.Close()

	// Seed the document with "Hello"
	seedOp := Operation{
		Type:     InsertOperation,
		Position: 0,
		Text:     "Hello",
	}
	sendOperation(t, connA, seedOp)

	// B receives the raw operation broadcast confirming the seed was applied.
	seedResp := readMessage(t, connB, time.Second)
	var seedOpReceived Operation
	if err := json.Unmarshal(seedResp, &seedOpReceived); err != nil {
		t.Fatalf("failed to parse seed broadcast on B: %v", err)
	}

	// Send an insert with empty text
	emptyOp := Operation{
		Type:     InsertOperation,
		Position: 2,
		Text:     "",
	}
	sendOperation(t, connA, emptyOp)

	// Client A should receive an error response
	connA.SetReadDeadline(time.Now().Add(time.Second))
	_, msg, err := connA.ReadMessage()
	if err != nil {
		t.Fatalf("client A did not receive an error response: %v", err)
	}

	var errResp map[string]interface{}
	if err := json.Unmarshal(msg, &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	if errResp["type"] != "error" {
		t.Errorf("expected error response type, got '%v'", errResp["type"])
	}
	errMsg, _ := errResp["error"].(string)
	if !strings.Contains(errMsg, "empty") {
		t.Errorf("expected error about empty text, got '%v'", errResp["error"])
	}
}

// TestIntegrationRejectInvalidOperationType verifies that the server rejects
// an operation with an unrecognized type.
//
// Operation: {"type": "update"}
// Expected: error response with "invalid operation type", no broadcast
func TestIntegrationRejectInvalidOperationType(t *testing.T) {
	hub := NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", hub.HandleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	connA := connectClient(t, server.URL, "doc-invalid-type")
	defer connA.Close()

	connB := connectClient(t, server.URL, "doc-invalid-type")
	defer connB.Close()

	// Send an operation with an invalid type
	invalidTypeOp := Operation{
		Type:     OperationType("update"),
		Position: 0,
		Text:     "Hello",
	}
	sendOperation(t, connA, invalidTypeOp)

	// Client A should receive an error response
	connA.SetReadDeadline(time.Now().Add(time.Second))
	_, msgA, err := connA.ReadMessage()
	if err != nil {
		t.Fatalf("client A did not receive an error response: %v", err)
	}

	var errResp map[string]interface{}
	if err := json.Unmarshal(msgA, &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	if errResp["type"] != "error" {
		t.Errorf("expected error response type, got '%v'", errResp["type"])
	}
	if !strings.Contains(errResp["error"].(string), "invalid operation type") {
		t.Errorf("expected 'invalid operation type' error, got '%v'", errResp["error"])
	}

	// Client B should NOT receive any broadcast
	expectNoMessage(t, connB, "client B (invalid type)", 200*time.Millisecond)
}

// TestIntegrationRejectNegativePosition verifies that the server rejects an
// INSERT operation with a negative position.
func TestIntegrationRejectNegativePosition(t *testing.T) {
	hub := NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", hub.HandleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	connA := connectClient(t, server.URL, "doc-neg-pos")
	defer connA.Close()

	connB := connectClient(t, server.URL, "doc-neg-pos")
	defer connB.Close()

	// Seed the document with "Hello"
	seedOp := Operation{
		Type:     InsertOperation,
		Position: 0,
		Text:     "Hello",
	}
	sendOperation(t, connA, seedOp)

	// B receives the raw operation broadcast confirming the seed was applied.
	seedResp := readMessage(t, connB, time.Second)
	var seedOpReceived Operation
	if err := json.Unmarshal(seedResp, &seedOpReceived); err != nil {
		t.Fatalf("failed to parse seed broadcast on B: %v", err)
	}

	// Send an insert with negative position
	negOp := Operation{
		Type:     InsertOperation,
		Position: -1,
		Text:     "X",
	}
	sendOperation(t, connA, negOp)

	// Client A should receive an error response
	connA.SetReadDeadline(time.Now().Add(time.Second))
	_, msg, err := connA.ReadMessage()
	if err != nil {
		t.Fatalf("client A did not receive an error response: %v", err)
	}

	var errResp map[string]interface{}
	if err := json.Unmarshal(msg, &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	if errResp["type"] != "error" {
		t.Errorf("expected error response type, got '%v'", errResp["type"])
	}
	if !strings.Contains(errResp["error"].(string), "position out of bounds") {
		t.Errorf("expected 'position out of bounds' error, got '%v'", errResp["error"])
	}
}

// TestIntegrationValidInsertNotRejected verifies that a valid insert operation
// is accepted and not rejected.
func TestIntegrationValidInsertNotRejected(t *testing.T) {
	hub := NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", hub.HandleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	connA := connectClient(t, server.URL, "doc-valid")
	defer connA.Close()

	connB := connectClient(t, server.URL, "doc-valid")
	defer connB.Close()

	// Send a valid insert
	op := Operation{
		Type:     InsertOperation,
		Position: 0,
		Text:     "Hello",
	}
	sendOperation(t, connA, op)

	// Client B should receive the raw operation broadcast (not an error)
	got := readMessage(t, connB, time.Second)

	var receivedOp Operation
	if err := json.Unmarshal(got, &receivedOp); err != nil {
		t.Fatalf("failed to parse broadcast operation JSON: %v", err)
	}
	if receivedOp.Type != InsertOperation {
		t.Errorf("expected operation type 'insert', got '%v'", receivedOp.Type)
	}
	if receivedOp.Position != 0 {
		t.Errorf("expected position 0, got %d", receivedOp.Position)
	}
	if receivedOp.Text != "Hello" {
		t.Errorf("expected text 'Hello', got '%s'", receivedOp.Text)
	}
}

// TestIntegrationRejectDeleteRangeExceedsContent verifies that the server
// rejects a DELETE operation where position + length exceeds the content
// length.
//
// This is the exact example from the task specification:
//
//	Document: "Hello" (length 5)
//	Delete: position = 3, length = 10
//	Reject because: 3 + 10 > 5
//
// Expected: error response with "exceeds", no broadcast
func TestIntegrationRejectDeleteRangeExceedsContent(t *testing.T) {
	hub := NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", hub.HandleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	connA := connectClient(t, server.URL, "doc-delete-exceeds")
	defer connA.Close()

	connB := connectClient(t, server.URL, "doc-delete-exceeds")
	defer connB.Close()

	// Seed the document with "Hello"
	seedOp := Operation{
		Type:     InsertOperation,
		Position: 0,
		Text:     "Hello",
	}
	sendOperation(t, connA, seedOp)

	// B receives the raw operation broadcast confirming the seed was applied.
	seedResp := readMessage(t, connB, time.Second)
	var seedOpReceived Operation
	if err := json.Unmarshal(seedResp, &seedOpReceived); err != nil {
		t.Fatalf("failed to parse seed broadcast on B: %v", err)
	}
	if seedOpReceived.Type != InsertOperation {
		t.Fatalf("expected insert operation for seed on B, got '%v'", seedOpReceived.Type)
	}
	if seedOpReceived.Position != 0 {
		t.Fatalf("expected position 0 for seed, got %d", seedOpReceived.Position)
	}
	if seedOpReceived.Text != "Hello" {
		t.Fatalf("expected text 'Hello' for seed, got '%s'", seedOpReceived.Text)
	}

	// Send a DELETE with position=3, length=10 (3 + 10 > 5)
	deleteOp := Operation{
		Type:     DeleteOperation,
		Position: 3,
		Length:   10,
	}
	sendOperation(t, connA, deleteOp)

	// Client A should receive an error response (the only message A gets)
	connA.SetReadDeadline(time.Now().Add(time.Second))
	_, msgA, err := connA.ReadMessage()
	if err != nil {
		t.Fatalf("client A did not receive an error response: %v", err)
	}

	var errResp map[string]interface{}
	if err := json.Unmarshal(msgA, &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	if errResp["type"] != "error" {
		t.Errorf("expected error response type, got '%v'", errResp["type"])
	}
	errMsg, _ := errResp["error"].(string)
	if !strings.Contains(errMsg, "exceeds") {
		t.Errorf("expected error about exceeding content length, got '%v'", errResp["error"])
	}

	// Client B should NOT receive any broadcast for the rejected operation
	expectNoMessage(t, connB, "client B (delete range exceeds)", 200*time.Millisecond)
}

// TestIntegrationValidDeleteNotRejected verifies that a valid DELETE operation
// is accepted and not rejected.
//
// Document: "Hello" (length 5)
// Delete: position = 0, length = 5
// Expected: raw delete operation broadcast (entire content deleted)
func TestIntegrationValidDeleteNotRejected(t *testing.T) {
	hub := NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", hub.HandleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	connA := connectClient(t, server.URL, "doc-valid-delete")
	defer connA.Close()

	connB := connectClient(t, server.URL, "doc-valid-delete")
	defer connB.Close()

	// Seed the document with "Hello"
	seedOp := Operation{
		Type:     InsertOperation,
		Position: 0,
		Text:     "Hello",
	}
	sendOperation(t, connA, seedOp)

	// B receives the seed operation broadcast
	seedResp := readMessage(t, connB, time.Second)
	var seedReceived Operation
	if err := json.Unmarshal(seedResp, &seedReceived); err != nil {
		t.Fatalf("failed to parse seed broadcast on B: %v", err)
	}

	// Send a valid DELETE: position=0, length=5 (0 + 5 == 5, not > 5)
	deleteOp := Operation{
		Type:     DeleteOperation,
		Position: 0,
		Length:   5,
	}
	sendOperation(t, connA, deleteOp)

	// Client B should receive the raw delete operation broadcast
	got := readMessage(t, connB, time.Second)

	var receivedOp Operation
	if err := json.Unmarshal(got, &receivedOp); err != nil {
		t.Fatalf("failed to parse broadcast operation JSON: %v", err)
	}
	if receivedOp.Type != DeleteOperation {
		t.Errorf("expected operation type 'delete', got '%v'", receivedOp.Type)
	}
	if receivedOp.Position != 0 {
		t.Errorf("expected position 0, got %d", receivedOp.Position)
	}
	if receivedOp.Length != 5 {
		t.Errorf("expected length 5, got %d", receivedOp.Length)
	}
}
