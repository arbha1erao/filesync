package main

import (
	"log"
	"net/http"
	"os"

	"github.com/arbha1erao/filesync/server"
)

func main() {
	server := server.NewServer()

	if err := os.Mkdir("server_storage", 0755); err != nil && !os.IsExist(err) {
		log.Fatalf("[FATAL] Error creating server storage directory: %v", err)
	}

	go server.BroadcastFileChanges()

	http.HandleFunc("/ws", server.HandleConnections)
	log.Println("[INFO] Server starting on :8080")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("[FATAL] Server error: %v", err)
	}
}
