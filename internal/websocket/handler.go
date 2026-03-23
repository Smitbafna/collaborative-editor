package websocket

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// HandleWebSocket upgrades an HTTP connection to a WebSocket connection.
func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("Client connected from %s", r.RemoteAddr)

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Read error: %v", err)
			break
		}
		log.Printf("received message: %s", message)

		response := append([]byte("echo: "), message...)
		if err := conn.WriteMessage(messageType, response); err != nil {
			log.Printf("Write error: %v", err)
			break
		}
	}

	log.Printf("Client disconnected from %s", r.RemoteAddr)
}