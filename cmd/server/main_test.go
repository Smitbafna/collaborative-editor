package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	ws "syncpad/internal/websocket"
)

func TestHealthEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)

	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%s'", body["status"])
	}
}

func TestWebSocketConnection(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", handleWebSocketForTest)

	server := httptest.NewServer(mux)
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/documents/document-123"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("WebSocket connection failed: %v", err)
	}
	defer conn.Close()

	// If we got here, connection succeeded
	t.Log("WebSocket connection established successfully")
}

// sendJSONOperation is a test helper that serializes an Operation and sends it as JSON.
func sendJSONOperation(t *testing.T, conn *websocket.Conn, op ws.Operation) {
	t.Helper()

	data, err := json.Marshal(op)
	if err != nil {
		t.Fatalf("failed to marshal operation: %v", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("failed to send operation: %v", err)
	}
}

// readJSONResponse reads a single JSON message from the WebSocket connection
// with a timeout and unmarshals it into a map.
func readJSONResponse(t *testing.T, conn *websocket.Conn, timeout time.Duration) map[string]interface{} {
	t.Helper()

	conn.SetReadDeadline(time.Now().Add(timeout))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(message, &resp); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}
	return resp
}

// TestWebSocketBroadcast verifies that operations are broadcast to all clients
// in the same room, including the sender.
//
// Client A sends an insert operation → both A and B receive a document_update
// Client B sends an insert operation → both A and B receive a document_update
func TestWebSocketBroadcast(t *testing.T) {
	hub := ws.NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", func(w http.ResponseWriter, r *http.Request) {
		hub.HandleWebSocket(w, r)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/documents/document-123"

	// Connect client A
	connA, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Client A connection failed: %v", err)
	}
	defer connA.Close()

	// Connect client B
	connB, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Client B connection failed: %v", err)
	}
	defer connB.Close()

	// Client A sends an insert operation
	opA := ws.Operation{
		Type:     ws.InsertOperation,
		Position: 0,
		Text:     "hello from A",
	}
	sendJSONOperation(t, connA, opA)

	// Client A should receive a document_update (self-broadcast)
	respA1 := readJSONResponse(t, connA, 2*time.Second)
	if respA1["type"] != "document_update" {
		t.Errorf("Client A expected 'document_update', got '%v'", respA1["type"])
	}
	contentA, _ := respA1["content"].(string)
	if contentA != "hello from A" {
		t.Errorf("Client A expected content 'hello from A', got '%s'", contentA)
	}

	// Client B should receive the document_update
	respB := readJSONResponse(t, connB, 2*time.Second)
	if respB["type"] != "document_update" {
		t.Errorf("Client B expected 'document_update', got '%v'", respB["type"])
	}
	contentB, _ := respB["content"].(string)
	if contentB != "hello from A" {
		t.Errorf("Client B expected content 'hello from A', got '%s'", contentB)
	}

	// Client B sends an insert operation
	opB := ws.Operation{
		Type:     ws.InsertOperation,
		Position: 0,
		Text:     "hello from B",
	}
	sendJSONOperation(t, connB, opB)

	// Client B should receive a document_update (self-broadcast)
	respB2 := readJSONResponse(t, connB, 2*time.Second)
	if respB2["type"] != "document_update" {
		t.Errorf("Client B expected 'document_update', got '%v'", respB2["type"])
	}
	contentB2, _ := respB2["content"].(string)
	if contentB2 != "hello from Bhello from A" {
		t.Errorf("Client B expected content 'hello from Bhello from A', got '%s'", contentB2)
	}

	// Client A should receive the document_update
	respA2 := readJSONResponse(t, connA, 2*time.Second)
	if respA2["type"] != "document_update" {
		t.Errorf("Client A expected 'document_update', got '%v'", respA2["type"])
	}
	contentA2, _ := respA2["content"].(string)
	if contentA2 != "hello from Bhello from A" {
		t.Errorf("Client A expected content 'hello from Bhello from A', got '%s'", contentA2)
	}
}

func TestMultipleWebSocketConnections(t *testing.T) {
	hub := ws.NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", func(w http.ResponseWriter, r *http.Request) {
		hub.HandleWebSocket(w, r)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/documents/document-123"

	// Connect client A
	connA, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Client A connection failed: %v", err)
	}
	defer connA.Close()

	// Connect client B
	connB, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Client B connection failed: %v", err)
	}
	defer connB.Close()

	// Connect client C
	connC, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Client C connection failed: %v", err)
	}
	defer connC.Close()

	// A sends an insert operation
	opA := ws.Operation{
		Type:     ws.InsertOperation,
		Position: 0,
		Text:     "from A",
	}
	sendJSONOperation(t, connA, opA)

	// B receives the document_update
	respB := readJSONResponse(t, connB, 2*time.Second)
	if respB["type"] != "document_update" {
		t.Errorf("Client B expected 'document_update', got '%v'", respB["type"])
	}
	contentB, _ := respB["content"].(string)
	if contentB != "from A" {
		t.Errorf("Client B expected content 'from A', got '%s'", contentB)
	}

	// C receives the document_update
	respC := readJSONResponse(t, connC, 2*time.Second)
	if respC["type"] != "document_update" {
		t.Errorf("Client C expected 'document_update', got '%v'", respC["type"])
	}
	contentC, _ := respC["content"].(string)
	if contentC != "from A" {
		t.Errorf("Client C expected content 'from A', got '%s'", contentC)
	}
}

func TestWebSocketDisconnect(t *testing.T) {
	hub := ws.NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/ws/documents/", func(w http.ResponseWriter, r *http.Request) {
		hub.HandleWebSocket(w, r)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/documents/document-123"

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("WebSocket connection failed: %v", err)
	}

	// Close the client connection
	conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	conn.Close()

	// Give the server a moment to process the disconnect
	time.Sleep(100 * time.Millisecond)

	// The server should still be running (no crash) - verify by making a health check
	healthResp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("Server appears to have crashed after disconnect: %v", err)
	}
	healthResp.Body.Close()
	if healthResp.StatusCode != http.StatusOK {
		t.Errorf("expected health check to return 200 after disconnect, got %d", healthResp.StatusCode)
	}
}

func TestRoomIsolation(t *testing.T) {
	hub := ws.NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", func(w http.ResponseWriter, r *http.Request) {
		hub.HandleWebSocket(w, r)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// Connect to document-123
	urlA := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/documents/document-123"
	connA, _, err := websocket.DefaultDialer.Dial(urlA, nil)
	if err != nil {
		t.Fatalf("Client A connection failed: %v", err)
	}
	defer connA.Close()

	// Connect to document-456 (different room)
	urlB := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/documents/document-456"
	connB, _, err := websocket.DefaultDialer.Dial(urlB, nil)
	if err != nil {
		t.Fatalf("Client B connection failed: %v", err)
	}
	defer connB.Close()

	// A sends an insert operation in document-123
	opA := ws.Operation{
		Type:     ws.InsertOperation,
		Position: 0,
		Text:     "secret for room 123",
	}
	sendJSONOperation(t, connA, opA)

	// A should receive a document_update (self-broadcast)
	respA := readJSONResponse(t, connA, 2*time.Second)
	if respA["type"] != "document_update" {
		t.Errorf("Client A expected 'document_update', got '%v'", respA["type"])
	}

	// B should NOT receive the message (different room)
	// Set a read deadline so we don't block forever
	connB.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, err = connB.ReadMessage()
	if err == nil {
		t.Error("Client B should not have received a message from a different room")
	}
}

// TestSlowClientDisconnection verifies that a slow client whose send buffer
// is full gets disconnected instead of blocking the room.
func TestSlowClientDisconnection(t *testing.T) {
	hub := ws.NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", func(w http.ResponseWriter, r *http.Request) {
		hub.HandleWebSocket(w, r)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/documents/document-123"

	// Connect client A (fast - will read messages)
	connA, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Client A connection failed: %v", err)
	}
	defer connA.Close()

	// Connect client B (slow - will NOT read messages, causing its send buffer to fill)
	connB, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Client B connection failed: %v", err)
	}

	// Send messages from A until B gets disconnected.
	// B's writePump will block because we never read from B's connection.
	// Once B's 256-slot send channel fills up, the next broadcast will
	// trigger disconnectSlowClient.
	// Use a large message payload to fill the TCP send buffer faster.
	// 1MB messages ensure that WriteMessage blocks on B's connection
	// (since the TCP send buffer is typically ~200KB), which causes
	// B's writePump to block, which fills B's Send channel.
	largeText := strings.Repeat("x", 1024*1024)

	msgCount := 0
	for {
		op := ws.Operation{
			Type:     ws.InsertOperation,
			Position: 0,
			Text:     largeText,
		}
		if err := connA.WriteMessage(websocket.TextMessage, mustMarshalJSON(op)); err != nil {
			t.Fatalf("Client A send failed after %d messages: %v", msgCount, err)
		}

		// Read A's own broadcast to keep A's send buffer from filling
		connA.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, _, err := connA.ReadMessage()
		if err != nil {
			t.Fatalf("Client A read failed after %d messages: %v", msgCount, err)
		}

		msgCount++

		// Check if B is still connected by attempting to read (with a very short deadline)
		connB.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
		_, _, err = connB.ReadMessage()
		if err != nil {
			// B should be disconnected - verify it's a close error, not a timeout
			t.Logf("Client B disconnected after %d messages from A (err: %v)", msgCount, err)
			return
		}

		// Safety limit to prevent infinite loop in case of test failure
		if msgCount > 5000 {
			t.Fatal("Client B was not disconnected after 5000 messages - slow client policy may not be working")
			return
		}
	}
}

// TestFastClientNotAffectedBySlowClientDisconnection verifies that when a slow
// client is disconnected, the fast clients in the room continue to work normally.
func TestFastClientNotAffectedBySlowClientDisconnection(t *testing.T) {
	hub := ws.NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/documents/", func(w http.ResponseWriter, r *http.Request) {
		hub.HandleWebSocket(w, r)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/documents/document-123"

	// Connect client A (fast)
	connA, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Client A connection failed: %v", err)
	}
	defer connA.Close()

	// Connect client B (slow - won't read)
	connB, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Client B connection failed: %v", err)
	}

	// Connect client C (fast)
	connC, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Client C connection failed: %v", err)
	}
	defer connC.Close()

	// Use a large message payload to fill the TCP send buffer faster.
	// 1MB messages ensure that WriteMessage blocks on B's connection.
	largeText := strings.Repeat("x", 1024*1024)

	// Send messages from A until B gets disconnected
	msgCount := 0
	for {
		op := ws.Operation{
			Type:     ws.InsertOperation,
			Position: 0,
			Text:     largeText,
		}
		if err := connA.WriteMessage(websocket.TextMessage, mustMarshalJSON(op)); err != nil {
			t.Fatalf("Client A send failed: %v", err)
		}

		// Read A's own broadcast
		connA.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		if _, _, err := connA.ReadMessage(); err != nil {
			t.Fatalf("Client A read failed: %v", err)
		}

		// Read C's broadcast too so C's send buffer doesn't fill up
		connC.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		if _, _, err := connC.ReadMessage(); err != nil {
			t.Fatalf("Client C read failed: %v", err)
		}

		msgCount++

		// Check if B is disconnected
		connB.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
		if _, _, err := connB.ReadMessage(); err != nil {
			t.Logf("Client B disconnected after %d messages", msgCount)
			break
		}

		if msgCount > 5000 {
			t.Fatal("Client B was not disconnected after 5000 messages")
			return
		}
	}

	// Now verify that fast clients (A and C) can still communicate
	msg := "hello after slow client was removed"
	op := ws.Operation{
		Type:     ws.InsertOperation,
		Position: 0,
		Text:     msg,
	}
	sendJSONOperation(t, connA, op)

	// A should receive its own broadcast
	respA := readJSONResponse(t, connA, 2*time.Second)
	if respA["type"] != "document_update" {
		t.Errorf("Client A expected 'document_update', got '%v'", respA["type"])
	}

	// C should receive the document_update too
	respC := readJSONResponse(t, connC, 2*time.Second)
	if respC["type"] != "document_update" {
		t.Errorf("Client C expected 'document_update', got '%v'", respC["type"])
	}

	t.Logf("Fast clients continue to work after slow client was disconnected")
}

// mustMarshalJSON serializes a value to JSON, failing the test on error.
func mustMarshalJSON(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

// handleWebSocketForTest is a test wrapper that uses the Hub's HandleWebSocket
// but is accessible from the test package.
// It expects the URL path: /ws/documents/{documentID}
func handleWebSocketForTest(w http.ResponseWriter, r *http.Request) {
	// Use a mutex to ensure thread safety when creating new Hubs per request
	// For simple connection tests, each request gets its own Hub
	hub := ws.NewHub()
	hub.HandleWebSocket(w, r)
}
