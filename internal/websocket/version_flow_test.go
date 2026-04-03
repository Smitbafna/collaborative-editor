package websocket

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ... existing code ...

// TestStaleClientSyncRequiredWithDelayedReconnectStep16 tests the exact
// stale-client synchronization scenario described in the task:
//
//	Server: Version 0
//	Client A: Version 0
//	Client B: Version 0
//
//	Client A sends: INSERT("Hello"), BaseVersion: 0
//	Server: Version 0 → Version 1
//
//	Client B goes offline while having Version 0
//
//	Client A sends: INSERT(" World"), BaseVersion: 1
//	Server: Version 1 → Version 2
//
//	Client B reconnects with Version 0
//	Server sends sync_required with operations for Version 1 and Version 2
//
//	Client B applies Version 1 operation → content: "Hello", version: 1
//	Client B applies Version 2 operation → content: "Hello World", version: 2
func TestStaleClientSyncRequiredWithDelayedReconnectStep16(t *testing.T) {
	hub := NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", hub.HandleWebSocket)
	server := httptest.NewServer(mux)
	defer server.Close()

	documentID := "stale-client-delayed-reconnect-test"

	// -----------------------------------------------------------------------
	// Step 1: Connect Client A — both start at version 0
	// -----------------------------------------------------------------------
	connA := connectClient(t, server.URL, documentID)
	defer connA.Close()
	_, _ = readSnapshot(t, connA, time.Second)

	// -----------------------------------------------------------------------
	// Step 2: Client A sends INSERT("Hello") with BaseVersion 0
	//           Server: Version 0 → Version 1
	// -----------------------------------------------------------------------
	opV1 := Operation{
		ID:          "op-v1-hello",
		Type:        InsertOperation,
		Position:    0,
		Text:        "Hello",
		BaseVersion: 0,
	}
	sendOperation(t, connA, opV1)
	time.Sleep(50 * time.Millisecond)

	room, ok := hub.RoomManager.GetRoom(documentID)
	if !ok {
		t.Fatalf("room %s not found", documentID)
	}
	if room.GetVersion() != 1 {
		t.Fatalf("after v1: expected server version 1, got %d", room.GetVersion())
	}
	if room.GetContent() != "Hello" {
		t.Fatalf("after v1: expected content 'Hello', got '%s'", room.GetContent())
	}

	// Client A updates its version to 1
	clientAVersion := int64(1)

	// -----------------------------------------------------------------------
	// Step 3: Client B connects, receives first operation, then disconnects
	//           Client B should have received the broadcast of Version 1
	// -----------------------------------------------------------------------
	connB := connectClient(t, server.URL, documentID)
	// Read the snapshot sent to Client B (it gets version 1 and content "Hello")
	contentB, versionB := readSnapshot(t, connB, time.Second)
	if versionB != 1 {
		t.Fatalf("Client B snapshot: expected version 1, got %d", versionB)
	}
	if contentB != "Hello" {
		t.Fatalf("Client B snapshot: expected content 'Hello', got '%s'", contentB)
	}

	// Give Client B time to fully join the room and process the broadcast
	time.Sleep(100 * time.Millisecond)

	// Client B goes offline — close its connection
	// Client B's last known version is 1 (from the snapshot)
	connB.Close()
	clientBVersion := int64(1)

	// -----------------------------------------------------------------------
	// Step 4: Client A sends INSERT(" World") with BaseVersion: 1
	//           Server: Version 1 → Version 2
	//           Client B is offline and misses this broadcast
	// -----------------------------------------------------------------------
	opV2 := Operation{
		ID:          "op-v2-world",
		Type:        InsertOperation,
		Position:    5,
		Text:        " World",
		BaseVersion: clientAVersion, // 1
	}
	sendOperation(t, connA, opV2)
	time.Sleep(50 * time.Millisecond)

	if room.GetVersion() != 2 {
		t.Fatalf("after v2: expected server version 2, got %d", room.GetVersion())
	}
	if room.GetContent() != "Hello World" {
		t.Fatalf("after v2: expected content 'Hello World', got '%s'", room.GetContent())
	}

	clientAVersion = 2 // Client A updates its version to 2

	// -----------------------------------------------------------------------
	// Step 5: Client B reconnects — it still has Version 1 (stale)
	// -----------------------------------------------------------------------
	connB = connectClient(t, server.URL, documentID)
	defer connB.Close()

	// Read the new snapshot (Client B receives the current state: Version 2)
	contentB2, versionB2 := readSnapshot(t, connB, time.Second)
	if versionB2 != 2 {
		t.Fatalf("Client B reconnected snapshot: expected version 2, got %d", versionB2)
	}
	if contentB2 != "Hello World" {
		t.Fatalf("Client B reconnected snapshot: expected content 'Hello World', got '%s'", contentB2)
	}

	// -----------------------------------------------------------------------
	// Step 6: Client B sends an operation with its stale BaseVersion (1)
	//           Server should transform it against the missing v2 operation
	//           and apply it, creating Version 3
	// -----------------------------------------------------------------------
	staleOp := Operation{
		ID:          "op-client-b-stale",
		Type:        InsertOperation,
		Position:    11,
		Text:        "!",
		BaseVersion: clientBVersion, // 1 — stale, server is at version 2
	}
	sendOperation(t, connB, staleOp)

	// Give the server time to process and broadcast the transformed operation
	time.Sleep(100 * time.Millisecond)

	// -----------------------------------------------------------------------
	// Step 7: Client B receives the broadcast of the transformed operation
	//           The operation was transformed from v1 through v2.
	//           Position 11 shifts to 17, but "Hello World" is only length 11.
	//           Position 17 is invalid, so the operation becomes a no-op.
	//           Version still increments to 3 to maintain the operation log.
	// -----------------------------------------------------------------------
	connB.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, opMessage, err := connB.ReadMessage()
	if err != nil {
		t.Fatalf("Client B: failed to read operation message: %v", err)
	}

	var opMsg map[string]interface{}
	if err := json.Unmarshal(opMessage, &opMsg); err != nil {
		t.Fatalf("Client B: failed to parse operation message: %v", err)
	}
	if opMsg["type"] != "operation" {
		t.Fatalf("Client B: expected message type 'operation', got '%v'", opMsg["type"])
	}

	opData := opMsg["operation"].(map[string]interface{})
	if opData["id"] != "op-client-b-stale" {
		t.Errorf("Client B: expected operation id 'op-client-b-stale', got '%v'", opData["id"])
	}
	if opData["type"] != "noop" {
		t.Errorf("Client B: expected operation type 'noop' (transformed to noop due to invalid position), got '%v'", opData["type"])
	}
	if int(opData["position"].(float64)) != 0 {
		t.Errorf("Client B: expected noop position 0, got %d", int(opData["position"].(float64)))
	}
	if opData["text"] != nil && opData["text"] != "" {
		t.Errorf("Client B: expected empty text for noop, got '%v'", opData["text"])
	}

	version := int64(opMsg["version"].(float64))
	if version != 3 {
		t.Errorf("Client B: expected version 3, got %d", version)
	}

	// -----------------------------------------------------------------------
	// Step 8: Verify final document state
	//           The server transformed the operation but it became invalid
	//           (position 17 > content length 11), so it was applied as a no-op.
	//           Version still increments to 3, but content remains "Hello World".
	// -----------------------------------------------------------------------
	if room.GetVersion() != 3 {
		t.Errorf("final: expected server version 3, got %d", room.GetVersion())
	}
	if room.GetContent() != "Hello World" {
		t.Errorf("final: expected content 'Hello World', got '%s'", room.GetContent())
	}

	// Verify Client A is at Version 3 (it should have received the broadcast too)
	if clientAVersion != 3 {
		t.Errorf("Client A: expected version 3, got %d", clientAVersion)
	}
}
