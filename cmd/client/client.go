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
//	{"id":"op-123","type":"insert","position":5,"text":" World"}
//
// For delete operations, "length" is used instead of "text":
//
//	{"id":"op-124","type":"delete","position":5,"length":6}
type operation struct {
	ID       string `json:"id"`
	Type     string `json:"type"`           // "insert" or "delete"
	Position int    `json:"position"`
	Text     string `json:"text,omitempty"` // used for insert
	Length   int    `json:"length,omitempty"` // used for delete
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

	// Send an insert operation: insert "Hello World" at position 0
	insertOp := operation{
		ID:       "op-1",
		Type:     "insert",
		Position: 0,
		Text:     "Hello World",
	}
	data, err := json.Marshal(insertOp)
	if err != nil {
		log.Fatal("failed to marshal operation:", err)
	}
	if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Fatal("write:", err)
	}
	fmt.Printf("Sent: %s\n", data)

	// Read response
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, response, err := c.ReadMessage()
	if err != nil {
		log.Fatal("read:", err)
	}
	fmt.Printf("Received: %s\n", response)

	// Send a delete operation: delete 5 characters at position 0
	deleteOp := operation{
		ID:       "op-2",
		Type:     "delete",
		Position: 0,
		Length:   5,
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

	// Wait for interrupt signal to cleanly close
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	<-interrupt
}
