package main

import (
	"log"

	"github.com/arbha1erao/filesync/client"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	client, err := client.NewClient()
	if err != nil {
		log.Fatalf("[FATAL] Error initializing client: %v", err)
	}
	defer client.Conn.Close()
	defer client.Watcher.Close()

	log.Println("[INFO] Connected to WebSocket server.")

	go client.ListenToServer()
	client.WatchFiles()
}
