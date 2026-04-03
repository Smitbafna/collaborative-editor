package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syncpad/internal/websocket"
	"syscall"
	"time"
)

func main() {
	mux := http.NewServeMux()

	hub := websocket.NewHub()

	// Serve the frontend
	mux.Handle("/", http.FileServer(http.Dir("./web")))

	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/ws/documents/", hub.HandleWebSocket)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start server in a goroutine so we can listen for signals
	go func() {
		log.Printf("Starting server on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("Received signal %v, shutting down gracefully...", sig)

	// Give outstanding requests a deadline to complete
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}