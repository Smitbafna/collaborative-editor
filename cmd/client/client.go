package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

// operation is the JSON message format sent from client to server.
// It uses the simpler flat format described in the task:
//
//	{"id":"op-123","type":"insert","position":5,"text":" World","base_version":5}
//
// For delete operations, "length" is used instead of "text":
//
//	{"id":"op-124","type":"delete","position":5,"length":6,"base_version":5}
type operation struct {
	ID          string `json:"id"`
	Type        string `json:"type"`            // "insert" or "delete"
	Position    int    `json:"position"`
	Text        string `json:"text,omitempty"`  // used for insert
	Length      int    `json:"length,omitempty"` // used for delete
	BaseVersion int64  `json:"base_version,omitempty"` // version this operation is based on
}

// serverMessage represents the envelope of any message received from the server.
// The "type" field determines which payload field to decode.
type serverMessage struct {
	Type           string          `json:"type"`
	Operation      json.RawMessage `json:"operation,omitempty"`
	Content        string          `json:"content,omitempty"`
	Version        int64           `json:"version,omitempty"`
	Message        string          `json:"message,omitempty"`
	CurrentVersion int64           `json:"current_version,omitempty"`
	Operations     []syncOperation `json:"operations,omitempty"`
}

// syncOperation pairs a version number with the operation that created it.
type syncOperation struct {
	Version   int64     `json:"version"`
	Operation operation `json:"operation"`
}

func main() {
	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws/documents/document-123"}
	fmt.Printf("Connecting to %s\n", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	fmt.Println("Connected to server")

	// Track the local version of the document
	// Starts at 0 (no operations applied yet)
	localVersion := int64(0)

	// expectedNextVersion tracks what version we expect to see next.
	// When we receive an operation with a version higher than this, we know
	// we missed some operations and need to request them from the server.
	expectedNextVersion := int64(1) // We expect version 1 next

	// Send an insert operation: insert "Hello World" at position 0
	insertOp := operation{
		ID:          "op-1",
		Type:        "insert",
		Position:    0,
		Text:        "Hello World",
		BaseVersion: localVersion,
	}
	data, err := json.Marshal(insertOp)
	if err != nil {
		log.Fatal("failed to marshal operation:", err)
	}
	if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Fatal("write:", err)
	}
	fmt.Printf("Sent: %s\n", data)

	// Read response — could be a document_snapshot, operation, or error
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, response, err := c.ReadMessage()
	if err != nil {
		log.Fatal("read:", err)
	}
	fmt.Printf("Received: %s\n", response)
	handleServerMessage(response, &localVersion, &expectedNextVersion)

	// Send a delete operation: delete 5 characters at position 0
	// Use the current local version as the BaseVersion
	deleteOp := operation{
		ID:          "op-2",
		Type:        "delete",
		Position:    0,
		Length:      5,
		BaseVersion: localVersion,
	}
	data, err = json.Marshal(deleteOp)
	if err != nil {
		log.Fatal("failed to marshal operation:", err)
	}
	if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Fatal("write:", err)
	}
	fmt.Printf("Sent: %s\n", data)

	// Read response
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, response, err = c.ReadMessage()
	if err != nil {
		log.Fatal("read:", err)
	}
	fmt.Printf("Received: %s\n", response)
	handleServerMessage(response, &localVersion, &expectedNextVersion)

	// Wait for interrupt signal to cleanly close
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	<-interrupt
}

// handleServerMessage processes a message received from the server and
// updates the client's local version if the message contains version information.
// It also prints a human-readable description of what was received.
//
// The localVersion parameter is updated in place when a version is received
// from the server (from either a document_snapshot or an operation broadcast).
// This ensures the client always has the correct version for its next operation.
func handleServerMessage(data []byte, localVersion *int64, expectedNextVersion *int64) {
	var msg serverMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		fmt.Printf("  (unable to parse message: %v)\n", err)
		return
	}
	switch msg.Type {
	case "document_snapshot":
		fmt.Printf("  → document snapshot (content=%q, version=%d)\n", msg.Content, msg.Version)
		// Update local version from the snapshot
		if msg.Version > 0 {
			*localVersion = msg.Version
			*expectedNextVersion = msg.Version + 1
			fmt.Printf("  → updated local version to %d\n", *localVersion)
		}
	case "operation":
		fmt.Printf("  → operation broadcast: %s\n", string(msg.Operation))
		// Check for version gap - if we receive a version higher than expected,
		// we missed some operations
		if msg.Version > *expectedNextVersion && *expectedNextVersion > 0 {
			fmt.Printf("  ⚠ VERSION GAP DETECTED: expected version %d, received %d\n", *expectedNextVersion, msg.Version)
			fmt.Printf("  → Requesting missing operations from server...\n")
			// Request missing operations from the server
			requestMissingOps(c, *expectedNextVersion-1)
		}

		// Update local version from the broadcast
		if msg.Version > *localVersion {
			*localVersion = msg.Version
			*expectedNextVersion = msg.Version + 1
			fmt.Printf("  → updated local version to %d\n", *localVersion)
		}
	case "sync_required":
		fmt.Printf("  → sync_required received\n")
		fmt.Printf("     current_version: %d\n", msg.CurrentVersion)
		fmt.Printf("     operations count: %d\n", len(msg.Operations))
		// Update local version to the server's current version
		*localVersion = msg.CurrentVersion
		*expectedNextVersion = msg.CurrentVersion + 1
		fmt.Printf("     → updated local version to %d\n", *localVersion)
	case "error":
		fmt.Printf("  → error: %s\n", msg.Message)
	default:
		fmt.Printf("  → unknown message type: %s\n", msg.Type)
	}
}

// requestMissingOps sends a request to the server for operations after the specified version.
func requestMissingOps(c *websocket.Conn, afterVersion int64) {
	req := operation{
		Type:        "request_missing_operations",
		BaseVersion: afterVersion,
	}
	data, err := json.Marshal(req)
	if err != nil {
		log.Printf("failed to marshal missing operations request: %v", err)
		return
	}
	if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("failed to send missing operations request: %v", err)
		return
	}
	fmt.Printf("  → Sent request for operations after version %d\n", afterVersion)
}

// applySyncOperations applies a list of operations received from a sync_required
// message to the local document state.
func applySyncOperations(ops []syncOperation) {
	fmt.Printf("  → Applying %d missing operations sequentially...\n", len(ops))
	for _, syncOp := range ops {
		fmt.Printf("     Applying version %d operation\n", syncOp.Version)
		// In a real implementation, you would apply the operation to your local document here
		// For now, we just track the version
	}
}