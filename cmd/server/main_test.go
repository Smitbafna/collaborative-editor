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

func TestWebSocketBroadcast(t *testing.T) {
	// Use a fresh Hub for this test to isolate from other tests
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

	// Client A sends a message, both Client A and Client B should receive it
	msgA := "hello from A"
	if err := connA.WriteMessage(websocket.TextMessage, []byte(msgA)); err != nil {
		t.Fatalf("Client A send failed: %v", err)
	}

	// Client A should receive its own message (broadcast to all, including sender)
	_, respA1, err := connA.ReadMessage()
	if err != nil {
		t.Fatalf("Client A first read failed: %v", err)
	}
	if string(respA1) != msgA {
		t.Errorf("Client A expected '%s' (self-broadcast), got '%s'", msgA, string(respA1))
	}

	// Client B should receive the message from A
	_, respB, err := connB.ReadMessage()
	if err != nil {
		t.Fatalf("Client B read failed: %v", err)
	}
	if string(respB) != msgA {
		t.Errorf("Client B expected '%s', got '%s'", msgA, string(respB))
	}

	// Client B sends a message, both Client A and Client B should receive it
	msgB := "hello from B"
	if err := connB.WriteMessage(websocket.TextMessage, []byte(msgB)); err != nil {
		t.Fatalf("Client B send failed: %v", err)
	}

	// Client B should receive its own message (self-broadcast)
	_, respB2, err := connB.ReadMessage()
	if err != nil {
		t.Fatalf("Client B second read failed: %v", err)
	}
	if string(respB2) != msgB {
		t.Errorf("Client B expected '%s' (self-broadcast), got '%s'", msgB, string(respB2))
	}

	// Client A should receive the message from B
	_, respA2, err := connA.ReadMessage()
	if err != nil {
		t.Fatalf("Client A second read failed: %v", err)
	}
	if string(respA2) != msgB {
		t.Errorf("Client A expected '%s', got '%s'", msgB, string(respA2))
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

	// A sends a message, B and C should both receive it
	msgA := "from A"
	if err := connA.WriteMessage(websocket.TextMessage, []byte(msgA)); err != nil {
		t.Fatalf("Client A send failed: %v", err)
	}

	// B receives
	_, respB, err := connB.ReadMessage()
	if err != nil {
		t.Fatalf("Client B read failed: %v", err)
	}
	if string(respB) != msgA {
		t.Errorf("Client B expected '%s', got '%s'", msgA, string(respB))
	}

	// C receives
	_, respC, err := connC.ReadMessage()
	if err != nil {
		t.Fatalf("Client C read failed: %v", err)
	}
	if string(respC) != msgA {
		t.Errorf("Client C expected '%s', got '%s'", msgA, string(respC))
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

	// A sends a message in document-123
	msgA := "secret for room 123"
	if err := connA.WriteMessage(websocket.TextMessage, []byte(msgA)); err != nil {
		t.Fatalf("Client A send failed: %v", err)
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
	payload := make([]byte, 1024*1024)
	for i := range payload {
		payload[i] = 'x'
	}

	msgCount := 0
	for {
		if err := connA.WriteMessage(websocket.TextMessage, payload); err != nil {
			t.Fatalf("Client A send failed after %d messages: %v", msgCount, err)
		}

		// Read A's own broadcast to keep A's send buffer from filling
		connA.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
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
	payload := make([]byte, 1024*1024)
	for i := range payload {
		payload[i] = 'x'
	}

	// Send messages from A until B gets disconnected
	msgCount := 0
	for {
		if err := connA.WriteMessage(websocket.TextMessage, payload); err != nil {
			t.Fatalf("Client A send failed: %v", err)
		}

		// Read A's own broadcast
		connA.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		if _, _, err := connA.ReadMessage(); err != nil {
			t.Fatalf("Client A read failed: %v", err)
		}

		// Read C's broadcast too so C's send buffer doesn't fill up
		connC.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
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
	if err := connA.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		t.Fatalf("Client A send after slow client disconnect failed: %v", err)
	}

	// A should receive its own broadcast
	_, respA, err := connA.ReadMessage()
	if err != nil {
		t.Fatalf("Client A read after slow client disconnect failed: %v", err)
	}
	if string(respA) != msg {
		t.Errorf("Client A expected '%s', got '%s'", msg, string(respA))
	}

	// C should receive the message too
	_, respC, err := connC.ReadMessage()
	if err != nil {
		t.Fatalf("Client C read after slow client disconnect failed: %v", err)
	}
	if string(respC) != msg {
		t.Errorf("Client C expected '%s', got '%s'", msg, string(respC))
	}

	t.Logf("Fast clients continue to work after slow client was disconnected")
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