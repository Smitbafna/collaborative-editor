package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"

	"github.com/gorilla/websocket"
)

func main() {
	message := "hello"
	if len(os.Args) > 1 {
		message = os.Args[1]
	}

	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
	fmt.Printf("Connecting to %s\n", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	fmt.Println("Connected to server")

	err = c.WriteMessage(websocket.TextMessage, []byte(message))
	if err != nil {
		log.Fatal("write:", err)
	}
	fmt.Printf("Sent: %s\n", message)

	// Read response
	_, response, err := c.ReadMessage()
	if err != nil {
		log.Fatal("read:", err)
	}
	fmt.Printf("Received: %s\n", response)

	// Wait for interrupt signal to cleanly close
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	<-interrupt
}