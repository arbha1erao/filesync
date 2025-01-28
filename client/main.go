package main

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
)

const (
	serverURL = "ws://localhost:8080/ws"
	syncDir   = "local_storage"
)

var (
	ignoreFiles   = make(map[string]bool)
	ignoreFilesMu sync.Mutex
)

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func main() {
	if err := os.MkdirAll(syncDir, 0755); err != nil {
		log.Fatalf("[ERROR] Could not create sync directory %s: %v\n", syncDir, err)
	}

	conn, _, err := websocket.DefaultDialer.Dial(serverURL, nil)
	if err != nil {
		log.Fatalf("[FATAL] Connection error: %v\n", err)
	}
	defer conn.Close()
	log.Println("[INFO] Connected to WebSocket server.")

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("[FATAL] Error setting up file watcher: %v\n", err)
	}
	defer watcher.Close()

	if err := watcher.Add(syncDir); err != nil {
		log.Fatalf("[FATAL] Failed to watch directory %s: %v\n", syncDir, err)
	}
	log.Printf("[INFO] Watching directory: %s\n", syncDir)

	go func() {
		for {
			var msg map[string]interface{}
			if err := conn.ReadJSON(&msg); err != nil {
				log.Printf("[ERROR] Error reading message from server: %v\n", err)
				return
			}
			handleServerMessage(conn, msg)
		}
	}()

	for event := range watcher.Events {
		if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
			log.Printf("[INFO] File change detected: %s (event: %v)\n", event.Name, event.Op)
			handleLocalChange(conn, event.Name)
		}
		if event.Op&fsnotify.Remove == fsnotify.Remove {
			log.Printf("[INFO] File deletion detected: %s\n", event.Name)
			handleLocalDelete(conn, filepath.Base(event.Name))
		}
	}
}

func handleLocalChange(conn *websocket.Conn, path string) {
	filename := filepath.Base(path)

	ignoreFilesMu.Lock()
	if ignoreFiles[filename] {
		log.Printf("[DEBUG] Ignoring local change for file: %s\n", filename)
		delete(ignoreFiles, filename)
		ignoreFilesMu.Unlock()
		return
	}
	ignoreFilesMu.Unlock()

	content, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[ERROR] Reading file %s: %v\n", path, err)
		return
	}

	log.Printf("[INFO] Sending upload request for file: %s\n", filename)
	err = conn.WriteJSON(map[string]interface{}{
		"operation": "upload",
		"filename":  filename,
		"content":   string(content),
	})
	if err != nil {
		log.Printf("[ERROR] Failed to send upload message for file %s: %v\n", filename, err)
	}
}

func handleLocalDelete(conn *websocket.Conn, filename string) {
	log.Printf("[INFO] Sending delete message for file: %s\n", filename)
	err := conn.WriteJSON(map[string]interface{}{
		"operation": "delete",
		"filename":  filename,
	})
	if err != nil {
		log.Printf("[ERROR] Failed to send delete message for file %s: %v\n", filename, err)
	}
}

func handleServerMessage(conn *websocket.Conn, msg map[string]interface{}) {
	filename, ok := msg["filename"].(string)
	if !ok {
		log.Println("[ERROR] Invalid server message format: missing filename.")
		return
	}

	operation, ok := msg["operation"].(string)
	if !ok {
		log.Println("[ERROR] Invalid server message format: missing operation.")
		return
	}

	switch operation {
	case "update":
		remoteTime := time.Unix(0, int64(msg["timestamp"].(float64)))
		localInfo, err := os.Stat(syncDir + "/" + filename)
		if err != nil || localInfo.ModTime().Before(remoteTime) {
			log.Printf("[INFO] Requesting file content for: %s (remote timestamp: %v)\n", filename, remoteTime)
			err = conn.WriteJSON(map[string]interface{}{
				"operation": "request",
				"filename":  filename,
			})
			if err != nil {
				log.Printf("[ERROR] Failed to send request for file %s: %v\n", filename, err)
			}
		}
	case "content":
		content, ok := msg["content"].(string)
		if !ok {
			log.Println("[ERROR] Invalid server message format: missing content.")
			return
		}

		ignoreFilesMu.Lock()
		ignoreFiles[filename] = true
		ignoreFilesMu.Unlock()

		err := os.WriteFile(syncDir+"/"+filename, []byte(content), 0644)
		if err != nil {
			log.Printf("[ERROR] Failed to write file %s: %v\n", filename, err)
		} else {
			log.Printf("[INFO] Updated file from server: %s\n", filename)
		}
	}
}
