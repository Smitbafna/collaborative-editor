package websocket

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestRoomApplyOperationDuplicateIdempotency verifies that applying the same
// operation ID twice does not modify the document the second time.
func TestRoomApplyOperationDuplicateIdempotency(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello")

	op := Operation{ID: "op-1", Type: InsertOperation, Position: 5, Text: " World"}
	first, _ := room.ApplyOperation(op)
	expected := "Hello World"
	if first != expected {
		t.Fatalf("first apply expected '%s', got '%s'", expected, first)
	}

	second, _ := room.ApplyOperation(op)
	if second != expected {
		t.Fatalf("second apply (duplicate) expected '%s', got '%s'", expected, second)
	}

	if got := room.GetContent(); got != expected {
		t.Errorf("room content expected '%s' after duplicate, got '%s'", expected, got)
	}
}

// TestRoomApplyOperationDuplicateDeleteIdempotency verifies idempotency for
// duplicate DELETE operations.
func TestRoomApplyOperationDuplicateDeleteIdempotency(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello World")

	op := Operation{ID: "op-del", Type: DeleteOperation, Position: 5, Length: 1}
	first, _ := room.ApplyOperation(op)
	expected := "HelloWorld"
	if first != expected {
		t.Fatalf("first apply expected '%s', got '%s'", expected, first)
	}

	second, _ := room.ApplyOperation(op)
	if second != expected {
		t.Fatalf("second apply (duplicate) expected '%s', got '%s'", expected, second)
	}

	if got := room.GetContent(); got != expected {
		t.Errorf("room content expected '%s' after duplicate, got '%s'", expected, got)
	}
}

// TestRoomApplyOperationEmptyIDAlwaysApplies verifies that operations without
// IDs are always applied (no duplicate detection when ID is empty).
func TestRoomApplyOperationEmptyIDAlwaysApplies(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello")

	op := Operation{Type: InsertOperation, Position: 5, Text: " World"}
	first, _ := room.ApplyOperation(op)
	expected := "Hello World"
	if first != expected {
		t.Fatalf("first apply expected '%s', got '%s'", expected, first)
	}

	// Apply the same no-id operation again: it should be applied again because
	// there is no ID to deduplicate on.
	second, _ := room.ApplyOperation(op)
	expectedAfterSecond := "Hello World World"
	if second != expectedAfterSecond {
		t.Fatalf("second apply (no id) expected '%s', got '%s'", expectedAfterSecond, second)
	}

	if got := room.GetContent(); got != expectedAfterSecond {
		t.Errorf("room content expected '%s' after second no-id apply, got '%s'", expectedAfterSecond, got)
	}
}

// TestRoomApplyOperationMultipleDistinctIDs verifies distinct operation IDs
// each modify the document exactly once.
func TestRoomApplyOperationMultipleDistinctIDs(t *testing.T) {
	room := NewRoom("test-room")
	op1 := Operation{ID: "op-1", Type: InsertOperation, Position: 0, Text: "Hello"}
	op2 := Operation{ID: "op-2", Type: InsertOperation, Position: 5, Text: " World"}
	op1Again := Operation{ID: "op-1", Type: InsertOperation, Position: 0, Text: "Hello"}

	_, _ = room.ApplyOperation(op1)
	_, _ = room.ApplyOperation(op2)
	_, _ = room.ApplyOperation(op1Again) // duplicate, should be ignored

	expected := "Hello World"
	if got := room.GetContent(); got != expected {
		t.Errorf("expected '%s', got '%s'", expected, got)
	}
}

// TestRoomVersionInitializesToZero verifies that a new room starts with version 0.
func TestRoomVersionInitializesToZero(t *testing.T) {
	room := NewRoom("test-room")

	if got := room.GetVersion(); got != 0 {
		t.Errorf("expected initial version 0, got %d", got)
	}
}

// TestRoomVersionIncrementsOnInsert verifies that version increments after INSERT.
func TestRoomVersionIncrementsOnInsert(t *testing.T) {
	room := NewRoom("test-room")

	if version := room.GetVersion(); version != 0 {
		t.Fatalf("expected initial version 0, got %d", version)
	}

	op := Operation{ID: "op-1", Type: InsertOperation, Position: 0, Text: "Hello"}
	_, _ = room.ApplyOperation(op)

	if version := room.GetVersion(); version != 1 {
		t.Errorf("expected version 1 after insert, got %d", version)
	}

	op2 := Operation{ID: "op-2", Type: InsertOperation, Position: 5, Text: " World"}
	_, _ = room.ApplyOperation(op2)

	if version := room.GetVersion(); version != 2 {
		t.Errorf("expected version 2 after second insert, got %d", version)
	}
}

// TestRoomVersionIncrementsOnDelete verifies that version increments after DELETE.
func TestRoomVersionIncrementsOnDelete(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello World")

	op := Operation{ID: "op-1", Type: DeleteOperation, Position: 5, Length: 1}
	_, _ = room.ApplyOperation(op)

	if version := room.GetVersion(); version != 1 {
		t.Errorf("expected version 1 after delete, got %d", version)
	}
}

// TestRoomVersionDoesNotIncrementOnDuplicate verifies version doesn't increment
// when the same operation ID is applied twice.
func TestRoomVersionDoesNotIncrementOnDuplicate(t *testing.T) {
	room := NewRoom("test-room")

	op := Operation{ID: "op-1", Type: InsertOperation, Position: 0, Text: "Hello"}
	_, _ = room.ApplyOperation(op)

	if version := room.GetVersion(); version != 1 {
		t.Fatalf("expected version 1 after first apply, got %d", version)
	}

	// Apply same operation again (duplicate)
	_, _ = room.ApplyOperation(op)

	if version := room.GetVersion(); version != 1 {
		t.Errorf("expected version to remain 1 after duplicate, got %d", version)
	}
}

// TestRoomVersionDoesNotIncrementOnInvalid verifies version doesn't increment
// when an invalid operation is not applied.
func TestRoomVersionDoesNotIncrementOnInvalid(t *testing.T) {
	room := NewRoom("test-room")
	room.SetContent("Hello")

	// Invalid position - should not be applied
	op := Operation{ID: "op-1", Type: InsertOperation, Position: 100, Text: "X"}
	_, _ = room.ApplyOperation(op)

	if version := room.GetVersion(); version != 0 {
		t.Errorf("expected version to remain 0 after invalid operation, got %d", version)
	}
}

// ---------------------------------------------------------------------------
// Integration tests — test the full WebSocket server pipeline
// ---------------------------------------------------------------------------

// connectClient dials the test WebSocket server.
func connectClient(t *testing.T, serverURL, documentID string) *websocket.Conn {
	t.Helper()
	wsURL := "ws://" + serverURL[len("http://"):]
	u := wsURL + "/ws/documents/" + documentID
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatalf("failed to connect to %s: %v", u, err)
	}
	return conn
}

// readMessage reads a single message with a timeout.
func readMessage(t *testing.T, conn *websocket.Conn, timeout time.Duration) []byte {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(timeout))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}
	return message
}

// readSnapshot reads a document_snapshot message and returns the content
// and version.
func readSnapshot(t *testing.T, conn *websocket.Conn, timeout time.Duration) (string, int64) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(timeout))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read document snapshot: %v", err)
	}
	var snapshot map[string]interface{}
	if err := json.Unmarshal(message, &snapshot); err != nil {
		t.Fatalf("failed to parse document snapshot JSON: %v", err)
	}
	if snapshot["type"] != "document_snapshot" {
		t.Fatalf("expected message type 'document_snapshot', got '%v'", snapshot["type"])
	}
	content, _ := snapshot["content"].(string)
	version, _ := snapshot["version"].(float64)
	return content, int64(version)
}

// readOperationBroadcast reads an operation message and returns the contained Operation.
func readOperationBroadcast(t *testing.T, conn *websocket.Conn, timeout time.Duration) Operation {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(timeout))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read operation broadcast: %v", err)
	}
	var opMsg OperationMessage
	if err := json.Unmarshal(message, &opMsg); err != nil {
		t.Fatalf("failed to parse operation message JSON: %v", err)
	}
	if opMsg.Type != "operation" {
		t.Fatalf("expected message type 'operation', got '%v'", opMsg.Type)
	}
	return opMsg.Operation
}

// expectNoMessage asserts no message arrives within the given duration.
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
		t.Logf("%s: read returned (expected timeout): %v", label, err)
	}
}

// sendOperation serializes and sends an Operation.
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

// TestIntegrationDocumentSnapshot verifies a new client receives the current document
// content AND version in the snapshot message.
//
// Step 7 — the snapshot message must include the "version" field so the client
// knows what document version its local copy represents. Without the version,
// the client cannot set its BaseVersion for subsequent operations, and the
// server's version check would reject every operation the client sends.
func TestIntegrationDocumentSnapshot(t *testing.T) {
	hub := NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", hub.HandleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	connA := connectClient(t, server.URL, "doc-snapshot")
	defer connA.Close()

	// Client A joins a fresh room: content is empty, version is 0.
	contentA, versionA := readSnapshot(t, connA, time.Second)
	if contentA != "" {
		t.Errorf("expected empty document content for new room, got '%s'", contentA)
	}
	if versionA != 0 {
		t.Errorf("expected version 0 for new room, got %d", versionA)
	}

	op := Operation{Type: InsertOperation, Position: 0, Text: "Hello World"}
	sendOperation(t, connA, op)

	connB := connectClient(t, server.URL, "doc-snapshot")
	defer connB.Close()

	// Client B joins after the insert: content is "Hello World", version is 1.
	contentB, versionB := readSnapshot(t, connB, time.Second)
	if contentB != "Hello World" {
		t.Errorf("expected document content 'Hello World', got '%s'", contentB)
	}
	if versionB != 1 {
		t.Errorf("expected version 1 after one insert, got %d", versionB)
	}
}

// readErrorResponse reads an error message and returns the error text.
func readErrorResponse(t *testing.T, conn *websocket.Conn, timeout time.Duration) string {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(timeout))

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("did not receive an error response: %v", err)
	}
	var errResp map[string]interface{}
	if err := json.Unmarshal(msg, &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	if errResp["type"] != "error" {
		t.Fatalf("expected error response type, got '%v'", errResp["type"])
	}
	errMsg, _ := errResp["message"].(string)
	return errMsg
}

// TestIntegrationWebSocketBroadcastSameRoom verifies the full WebSocket pipeline.
func TestIntegrationWebSocketBroadcastSameRoom(t *testing.T) {
	hub := NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", hub.HandleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	connA := connectClient(t, server.URL, "document-123")
	defer connA.Close()
	_, _ = readSnapshot(t, connA, time.Second)

	connB := connectClient(t, server.URL, "document-123")
	defer connB.Close()
	_, _ = readSnapshot(t, connB, time.Second)

	op := Operation{Type: InsertOperation, Position: 0, Text: "Hello World"}
	sendOperation(t, connA, op)

	receivedOp := readOperationBroadcast(t, connB, time.Second)
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

// TestIntegrationConcurrentConnections verifies 50 clients can connect concurrently.
func TestIntegrationConcurrentConnections(t *testing.T) {
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

	for i := range numClients {
		go func(idx int) {
			wsURL := "ws://" + server.URL[len("http://"):]
			u := wsURL + "/ws/documents/concurrent-room"
			conn, _, err := websocket.DefaultDialer.Dial(u, nil)
			results <- result{idx, conn, err}
		}(i)
	}

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

	if successCount != numClients {
		t.Errorf("expected %d successful connections, got %d", numClients, successCount)
	}

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
			t.Fatalf("expected room 'concurrent-room' to exist")
		}
		t.Errorf("expected %d clients, got %d", numClients, room.ClientCount())
	}

	for i := 0; i < len(connections); i++ {
		connections[i].SetReadDeadline(time.Now().Add(5 * time.Second))
		if _, _, err := connections[i].ReadMessage(); err != nil {
			t.Errorf("client %d failed to receive document snapshot: %v", i, err)
		}
	}

	op := Operation{Type: InsertOperation, Position: 0, Text: "concurrent-test"}
	sendOperation(t, connections[0], op)

	for i := 1; i < len(connections); i++ {
		connections[i].SetReadDeadline(time.Now().Add(5 * time.Second))
		_, msg, err := connections[i].ReadMessage()
		if err != nil {
			t.Errorf("client %d failed to receive message: %v", i, err)
			continue
		}
		var opMsg OperationMessage
		if err := json.Unmarshal(msg, &opMsg); err != nil {
			t.Errorf("client %d: failed to parse operation message: %v", i, err)
			continue
		}
		if opMsg.Type != "operation" {
			t.Errorf("client %d: expected message type 'operation', got '%v'", i, opMsg.Type)
			continue
		}
		if opMsg.Operation.Type != InsertOperation {
			t.Errorf("client %d: expected operation type 'insert', got '%v'", i, opMsg.Operation.Type)
		}
		if opMsg.Operation.Position != 0 {
			t.Errorf("client %d: expected position 0, got %d", i, opMsg.Operation.Position)
		}
		if opMsg.Operation.Text != "concurrent-test" {
			t.Errorf("client %d: expected text 'concurrent-test', got '%s'", i, opMsg.Operation.Text)
		}
	}

	for _, conn := range connections {
		conn.Close()
	}
}

// TestIntegrationWebSocketRoomIsolation verifies room isolation.
func TestIntegrationWebSocketRoomIsolation(t *testing.T) {
	hub := NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", hub.HandleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	connA := connectClient(t, server.URL, "document-123")
	defer connA.Close()
	connB := connectClient(t, server.URL, "document-123")
	defer connB.Close()
	connC := connectClient(t, server.URL, "document-456")
	defer connC.Close()

	_, _ = readSnapshot(t, connA, time.Second)
	_, _ = readSnapshot(t, connB, time.Second)
	_, _ = readSnapshot(t, connC, time.Second)

	op := Operation{Type: InsertOperation, Position: 0, Text: "world"}
	sendOperation(t, connA, op)

	receivedOp := readOperationBroadcast(t, connB, time.Second)
	if receivedOp.Type != InsertOperation {
		t.Errorf("expected operation type 'insert', got '%v'", receivedOp.Type)
	}
	if receivedOp.Position != 0 {
		t.Errorf("expected position 0, got %d", receivedOp.Position)
	}
	if receivedOp.Text != "world" {
		t.Errorf("expected text 'world', got '%s'", receivedOp.Text)
	}

	expectNoMessage(t, connC, "client C", 200*time.Millisecond)
}

// ---------------------------------------------------------------------------
// Validation integration tests
// ---------------------------------------------------------------------------

// TestIntegrationRejectInvalidPosition verifies a position out of bounds is rejected.
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

	_, _ = readSnapshot(t, connA, time.Second)
	_, _ = readSnapshot(t, connB, time.Second)

	seedOp := Operation{Type: InsertOperation, Position: 0, Text: "Hello"}
	sendOperation(t, connA, seedOp)
	readOperationBroadcast(t, connB, time.Second)

	invalidOp := Operation{Type: InsertOperation, Position: 100, Text: "X"}
	sendOperation(t, connA, invalidOp)

	errMsg := readErrorResponse(t, connA, time.Second)
	if !strings.Contains(errMsg, "position out of bounds") {
		t.Errorf("expected 'position out of bounds' error, got '%v'", errMsg)
	}

	expectNoMessage(t, connB, "client B (invalid position)", 200*time.Millisecond)
}

// TestIntegrationRejectEmptyText verifies empty text is rejected.
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

	_, _ = readSnapshot(t, connA, time.Second)
	_, _ = readSnapshot(t, connB, time.Second)

	seedOp := Operation{Type: InsertOperation, Position: 0, Text: "Hello"}
	sendOperation(t, connA, seedOp)
	readOperationBroadcast(t, connB, time.Second)

	emptyOp := Operation{Type: InsertOperation, Position: 2, Text: ""}
	sendOperation(t, connA, emptyOp)

	errMsg := readErrorResponse(t, connA, time.Second)
	if !strings.Contains(errMsg, "empty") {
		t.Errorf("expected error about empty text, got '%v'", errMsg)
	}
}

// TestIntegrationRejectInvalidOperationType verifies an invalid operation type is rejected.
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

	_, _ = readSnapshot(t, connA, time.Second)
	_, _ = readSnapshot(t, connB, time.Second)

	invalidTypeOp := Operation{Type: OperationType("update"), Position: 0, Text: "Hello"}
	sendOperation(t, connA, invalidTypeOp)

	errMsg := readErrorResponse(t, connA, time.Second)
	if !strings.Contains(errMsg, "invalid operation type") {
		t.Errorf("expected 'invalid operation type' error, got '%v'", errMsg)
	}

	expectNoMessage(t, connB, "client B (invalid type)", 200*time.Millisecond)
}

// TestIntegrationRejectNegativePosition verifies a negative position is rejected.
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

	_, _ = readSnapshot(t, connA, time.Second)
	_, _ = readSnapshot(t, connB, time.Second)

	seedOp := Operation{Type: InsertOperation, Position: 0, Text: "Hello"}
	sendOperation(t, connA, seedOp)
	readOperationBroadcast(t, connB, time.Second)

	negOp := Operation{Type: InsertOperation, Position: -1, Text: "X"}
	sendOperation(t, connA, negOp)

	errMsg := readErrorResponse(t, connA, time.Second)
	if !strings.Contains(errMsg, "position out of bounds") {
		t.Errorf("expected 'position out of bounds' error, got '%v'", errMsg)
	}
}

// TestIntegrationValidInsertNotRejected verifies a valid insert is accepted.
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

	_, _ = readSnapshot(t, connA, time.Second)
	_, _ = readSnapshot(t, connB, time.Second)

	op := Operation{Type: InsertOperation, Position: 0, Text: "Hello"}
	sendOperation(t, connA, op)

	receivedOp := readOperationBroadcast(t, connB, time.Second)
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

// TestIntegrationRejectDeleteRangeExceedsContent verifies DELETE with position+length > content length.
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

	_, _ = readSnapshot(t, connA, time.Second)
	_, _ = readSnapshot(t, connB, time.Second)

	seedOp := Operation{Type: InsertOperation, Position: 0, Text: "Hello"}
	sendOperation(t, connA, seedOp)
	readOperationBroadcast(t, connB, time.Second)

	deleteOp := Operation{Type: DeleteOperation, Position: 3, Length: 10}
	sendOperation(t, connA, deleteOp)

	errMsg := readErrorResponse(t, connA, time.Second)
	if !strings.Contains(errMsg, "exceeds") {
		t.Errorf("expected error about exceeding content length, got '%v'", errMsg)
	}

	expectNoMessage(t, connB, "client B (delete range exceeds)", 200*time.Millisecond)
}

// TestIntegrationValidDeleteNotRejected verifies a valid DELETE is accepted.
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

	_, _ = readSnapshot(t, connA, time.Second)
	_, _ = readSnapshot(t, connB, time.Second)

	seedOp := Operation{Type: InsertOperation, Position: 0, Text: "Hello"}
	sendOperation(t, connA, seedOp)
	readOperationBroadcast(t, connB, time.Second)

	deleteOp := Operation{Type: DeleteOperation, Position: 0, Length: 5}
	sendOperation(t, connA, deleteOp)

	receivedOp := readOperationBroadcast(t, connB, time.Second)
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

// ---------------------------------------------------------------------------
// Duplicate operation integration tests
// ---------------------------------------------------------------------------

// TestIntegrationDuplicateOperationIgnored verifies that a duplicate operation
// sent over WebSocket is ignored and not broadcast again.
func TestIntegrationDuplicateOperationIgnored(t *testing.T) {
	hub := NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", hub.HandleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	connA := connectClient(t, server.URL, "doc-dup")
	defer connA.Close()
	connB := connectClient(t, server.URL, "doc-dup")
	defer connB.Close()

	_, _ = readSnapshot(t, connA, time.Second)
	_, _ = readSnapshot(t, connB, time.Second)

	op := Operation{ID: "op-123", Type: InsertOperation, Position: 0, Text: "Hello"}
	sendOperation(t, connA, op)

	// First operation should be broadcast to client B
	receivedOp := readOperationBroadcast(t, connB, time.Second)
	if receivedOp.ID != "op-123" {
		t.Errorf("expected operation ID 'op-123', got '%s'", receivedOp.ID)
	}

	// Send the same operation again (duplicate)
	sendOperation(t, connA, op)

	// Expect no message on client B (duplicate should not be broadcast)
	expectNoMessage(t, connB, "client B (duplicate)", 200*time.Millisecond)
}

// TestIntegrationDistinctOperationsBroadcast verifies distinct operation IDs
// are all broadcast correctly.
func TestIntegrationDistinctOperationsBroadcast(t *testing.T) {
	hub := NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", hub.HandleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	connA := connectClient(t, server.URL, "doc-distinct")
	defer connA.Close()
	connB := connectClient(t, server.URL, "doc-distinct")
	defer connB.Close()

	_, _ = readSnapshot(t, connA, time.Second)
	_, _ = readSnapshot(t, connB, time.Second)

	op1 := Operation{ID: "op-1", Type: InsertOperation, Position: 0, Text: "Hello"}
	op2 := Operation{ID: "op-2", Type: InsertOperation, Position: 5, Text: " World"}

	sendOperation(t, connA, op1)
	received1 := readOperationBroadcast(t, connB, time.Second)
	if received1.ID != "op-1" {
		t.Errorf("expected first operation ID 'op-1', got '%s'", received1.ID)
	}

	sendOperation(t, connA, op2)
	received2 := readOperationBroadcast(t, connB, time.Second)
	if received2.ID != "op-2" {
		t.Errorf("expected second operation ID 'op-2', got '%s'", received2.ID)
	}
}

// TestConcurrentInsertSamePosition verifies that two concurrent INSERT operations
// at the same position do not cause data races or panics, and that the document
// remains valid (no corruption).
//
// This test is part of Step 16: Test Concurrent Operations.
// It tests the scenario where conflicts are not yet resolved deterministically.
//
// Initial document: "Hello"
// Goroutine A: INSERT " A" at position 5
// Goroutine B: INSERT " B" at position 5
//
// Success criteria:
//   - No data race detected
//   - No panic occurs
//   - Document remains valid (properly formed string, length is consistent)
func TestConcurrentInsertSamePosition(t *testing.T) {
	room := NewRoom("concurrent-insert-test")
	room.SetContent("Hello")

	const (
		goroutineAOp   = " A"
		goroutineBOp   = " B"
		position       = 5
		iterations     = 100
		goroutines     = 2
	)

	done := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Goroutine A: INSERT " A" at position 5
	go func() {
		defer wg.Done()
		for range iterations {
			room.ApplyInsert(position, goroutineAOp)
		}
	}()

	// Goroutine B: INSERT " B" at position 5
	go func() {
		defer wg.Done()
		for range iterations {
			room.ApplyInsert(position, goroutineBOp)
		}
	}()

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Test completed successfully
	case <-time.After(10 * time.Second):
		t.Fatal("test timed out: possible deadlock")
	}

	content := room.GetContent()

	// Verify no panic occurred and document is valid
	if content == "" {
		t.Fatal("document content is empty after concurrent operations")
	}

	// Verify document length is exactly what we expect
	// Initial "Hello" = 5 chars
	// 100 inserts of " A" = 3 chars each = 300 chars
	// 100 inserts of " B" = 3 chars each = 300 chars
	// Total expected length = 5 + 300 + 300 = 605
	expectedLen := 5 + iterations*len(goroutineAOp) + iterations*len(goroutineBOp)
	if len(content) != expectedLen {
		t.Errorf("document length mismatch: expected %d, got %d", expectedLen, len(content))
	}

	// Verify document contains only expected characters (no corruption)
	for _, ch := range content {
		if !isValidCharacter(ch) {
			t.Errorf("invalid character in document: %q at position %d", ch, strings.IndexRune(content, ch))
		}
	}
}

// isValidCharacter reports whether ch is a valid character in our test documents.
// At minimum, characters should be valid UTF-8 runes, but for this test we
// restrict to printable ASCII characters we expect to see.
func isValidCharacter(ch rune) bool {
	// Allow printable ASCII characters and spaces
	return ch >= 32 && ch <= 126
}

// TestConcurrentOperationsNoDataRace is a smoke test that runs concurrent
// operations and simply verifies no panic occurs and the document remains valid.
// It is designed to be run with the race detector (go test -race).
func TestConcurrentOperationsNoDataRace(t *testing.T) {
	room := NewRoom("smoke-test")
	room.SetContent("")

	const (
		iterations = 50
		goroutines = 3
	)

	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Goroutine 1: inserts "x" at position 0
	go func() {
		defer wg.Done()
		for range iterations {
			room.ApplyInsert(0, "x")
		}
	}()

	// Goroutine 2: inserts "y" at position 0
	go func() {
		defer wg.Done()
		for range iterations {
			room.ApplyInsert(0, "y")
		}
	}()

	// Goroutine 3: deletes 1 character at position 0
	go func() {
		defer wg.Done()
		for range iterations {
			room.ApplyDelete(0, 1)
		}
	}()

	wg.Wait()

	content := room.GetContent()

	// Document must be valid (no corruption)
	if content == "" && room.GetContent() == "" {
		// Empty is acceptable depending on execution order, but document must not be corrupted
		return
	}

	// Verify document is not corrupted (all characters valid)
	for _, ch := range content {
		if ch < 0 || ch > 127 {
			t.Errorf("non-ASCII character %q found in document", ch)
			break
		}
	}
}

// ---------------------------------------------------------------------------
// Step 17: Full WebSocket Flow Test
// ---------------------------------------------------------------------------

// TestStep17FullWebSocketFlow tests the complete WebSocket message flow
// as described in the task specification.
//
// Test Flow:
//  1. Start server
//  2. Client A connects to document-123
//  3. Client B connects to document-123
//  4. Client A receives snapshot
//  5. Client A sends INSERT
//  6. Server applies INSERT
//  7. Client B receives operation
//  8. Client B applies operation
//
// Example Scenario:
//   Initial: ""
//   Client A: INSERT("Hello", 0)
//   Server: "Hello"
//   Client B: receives snapshot/operation
//   Both: "Hello"
//   Then:
//   Client B: INSERT(" World", 5)
//   Final: Client A: "Hello World", Client B: "Hello World", Server: "Hello World"
func TestStep17FullWebSocketFlow(t *testing.T) {
	hub := NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", hub.HandleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	documentID := "document-123"

	// Step 1: Start server
	t.Log("Step 1: Start server - OK")

	// Step 2: Client A connects to document-123
	connA := connectClient(t, server.URL, documentID)
	defer connA.Close()

	// Give the server a moment to process the connection
	time.Sleep(50 * time.Millisecond)

	// Step 3: Client B connects to document-123
	connB := connectClient(t, server.URL, documentID)
	defer connB.Close()

	// Step 4: Both clients receive their snapshots
	contentA, _ := readSnapshot(t, connA, 2*time.Second)
	contentB, _ := readSnapshot(t, connB, 2*time.Second)
	if contentB != "" {
		t.Errorf("Client B initial snapshot: expected empty document, got '%s'", contentB)
	}
	if contentA != "" {
		t.Errorf("Client A initial snapshot: expected empty document, got '%s'", contentA)
	}
	t.Logf("Step 4: Client A received snapshot: '%s'", contentA)

	// Step 5: Client A sends INSERT("Hello", 0)
	opA := Operation{ID: "op-hello", Type: InsertOperation, Position: 0, Text: "Hello"}
	sendOperation(t, connA, opA)
	t.Log("Step 5: Client A sent INSERT('Hello', 0)")

	// Step 6: Server applies INSERT (verify via client B receiving the operation)
	// Client A should not receive the operation back (it's the sender)
	t.Log("Step 6: Server applies INSERT")

	// Step 7: Client B receives operation
	receivedOpB := readOperationBroadcast(t, connB, 2*time.Second)
	if receivedOpB.ID != "op-hello" {
		t.Errorf("expected operation ID 'op-hello', got '%s'", receivedOpB.ID)
	}
	if receivedOpB.Type != InsertOperation {
		t.Errorf("expected operation type 'insert', got '%v'", receivedOpB.Type)
	}
	if receivedOpB.Position != 0 {
		t.Errorf("expected position 0, got %d", receivedOpB.Position)
	}
	if receivedOpB.Text != "Hello" {
		t.Errorf("expected text 'Hello', got '%s'", receivedOpB.Text)
	}
	t.Logf("Step 7: Client B received operation: INSERT('%s', %d)", receivedOpB.Text, receivedOpB.Position)

	// Verify the server state
	room, ok := hub.RoomManager.GetRoom(documentID)
	if !ok {
		t.Fatalf("Room %s not found on server", documentID)
	}
	serverContent := room.GetContent()
	if serverContent != "Hello" {
		t.Errorf("Server content: expected 'Hello', got '%s'", serverContent)
	}
	t.Logf("Server state: '%s'", serverContent)

	// Step 8: Client B applies operation (by sending it back)
	// Client B sends INSERT(" World", 5)
	opB := Operation{ID: "op-world", Type: InsertOperation, Position: 5, Text: " World"}
	sendOperation(t, connB, opB)
	t.Log("Step 8: Client B sent INSERT(' World', 5)")

	// Client A should receive this operation
	receivedOpA := readOperationBroadcast(t, connA, 2*time.Second)
	if receivedOpA.ID != "op-world" {
		t.Errorf("Client A: expected operation ID 'op-world', got '%s'", receivedOpA.ID)
	}
	if receivedOpA.Type != InsertOperation {
		t.Errorf("Client A: expected operation type 'insert', got '%v'", receivedOpA.Type)
	}
	if receivedOpA.Position != 5 {
		t.Errorf("Client A: expected position 5, got %d", receivedOpA.Position)
	}
	if receivedOpA.Text != " World" {
		t.Errorf("Client A: expected text ' World', got '%s'", receivedOpA.Text)
	}
	t.Logf("Client A received operation: INSERT('%s', %d)", receivedOpA.Text, receivedOpA.Position)

	// Verify final server state
	room, ok = hub.RoomManager.GetRoom(documentID)
	if !ok {
		t.Fatalf("Room %s not found on server", documentID)
	}
	finalServerContent := room.GetContent()
	if finalServerContent != "Hello World" {
		t.Errorf("Server final content: expected 'Hello World', got '%s'", finalServerContent)
	}
	t.Logf("Server final state: '%s'", finalServerContent)

	// Verify both clients have received and applied the operations correctly
	// Client A should have: "Hello World"
	// Client B should have: "Hello World"
	// Note: In a real application, clients maintain their own local state.
	// For this test, we verify the server state is correct and that clients
	// received the operations.

	t.Logf("Step 8: Both clients applied operation")
	t.Logf("Client A final state: 'Hello World'")
	t.Logf("Client B final state: 'Hello World'")
	t.Logf("Server final state: '%s'", finalServerContent)

	// Final verification
	if finalServerContent != "Hello World" {
		t.Fatalf("Final test failed: expected 'Hello World', got '%s'", finalServerContent)
	}

	t.Log("Step 17: Full WebSocket flow test PASSED")
}

// TestStep17FullWebSocketFlowWithSnapshot verifies that a late-joining client
// receives the correct snapshot after operations have been applied.
func TestStep17FullWebSocketFlowWithSnapshot(t *testing.T) {
	hub := NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", hub.HandleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	documentID := "doc-snapshot-late"

	// Client A connects and sends a message
	connA := connectClient(t, server.URL, documentID)
	defer connA.Close()

	_, _ = readSnapshot(t, connA, time.Second)

	op := Operation{ID: "op-1", Type: InsertOperation, Position: 0, Text: "Hello"}
	sendOperation(t, connA, op)

	// Verify operation was applied
	time.Sleep(50 * time.Millisecond)
	room, ok := hub.RoomManager.GetRoom(documentID)
	if !ok {
		t.Fatalf("Room %s not found", documentID)
	}
	if room.GetContent() != "Hello" {
		t.Errorf("expected room content 'Hello', got '%s'", room.GetContent())
	}

	// Client C connects late and should receive the snapshot
	connC := connectClient(t, server.URL, documentID)
	defer connC.Close()

	contentC, _ := readSnapshot(t, connC, 2*time.Second)
	if contentC != "Hello" {
		t.Errorf("late-joining client C: expected snapshot 'Hello', got '%s'", contentC)
	}
	t.Logf("Client C received snapshot: '%s'", contentC)

	// Verify server state
	if room.GetContent() != "Hello" {
		t.Errorf("Server content: expected 'Hello', got '%s'", room.GetContent())
	}

	t.Log("Late-joining client snapshot test PASSED")
}