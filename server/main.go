package main

import (
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}

	clients   = make(map[*websocket.Conn]bool)
	clientsMu sync.Mutex

	fileState   = make(map[string]time.Time)
	fileStateMu sync.Mutex
)

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func main() {
	err := os.Mkdir("server_storage", 0755)
	if err != nil && !os.IsExist(err) {
		log.Fatalf("[FATAL] Error creating server storage directory: %v\n", err)
	}

	go broadcastFileChanges()

	http.HandleFunc("/ws", handleConnections)

	log.Println("[INFO] Server starting on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("[FATAL] Server error: %v\n", err)
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ERROR] WebSocket upgrade error: %v, from client: %s\n", err, r.RemoteAddr)
		return
	}
	defer ws.Close()

	clientsMu.Lock()
	clients[ws] = true
	clientsMu.Unlock()

	log.Printf("[INFO] New client connected: %s\n", r.RemoteAddr)

	files, err := os.ReadDir("server_storage")
	if err != nil {
		log.Printf("[ERROR] Reading server storage directory: %v\n", err)
		return
	}

	for _, f := range files {
		content, err := os.ReadFile("server_storage/" + f.Name())
		if err != nil {
			log.Printf("[ERROR] Reading file %s: %v\n", f.Name(), err)
			continue
		}
		err = ws.WriteJSON(map[string]interface{}{
			"filename": f.Name(),
			"content":  string(content),
		})
		if err != nil {
			log.Printf("[ERROR] Sending initial file to client: %v\n", err)
			break
		}
	}

	for {
		var msg map[string]interface{}
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("[ERROR] WebSocket read error: %v, from client: %s\n", err, r.RemoteAddr)
			clientsMu.Lock()
			delete(clients, ws)
			clientsMu.Unlock()
			break
		}

		log.Printf("[INFO] Received message from client %s: %v\n", r.RemoteAddr, msg)
		handleMessage(msg)
	}
}

func handleMessage(msg map[string]interface{}) {
	filename, ok := msg["filename"].(string)
	if !ok || filename == "" {
		log.Printf("[ERROR] Invalid or missing 'filename' field in message: %v\n", msg)
		return
	}

	operation, ok := msg["operation"].(string)
	if !ok || operation == "" {
		log.Printf("[ERROR] Invalid or missing 'operation' field in message: %v\n", msg)
		return
	}

	log.Printf("[INFO] Handling operation '%s' for file '%s'\n", operation, filename)

	switch operation {
	case "upload":
		contentStr, ok := msg["content"].(string)
		if !ok {
			log.Printf("[ERROR] Invalid or missing 'content' field in upload message: %v\n", msg)
			return
		}

		content := []byte(contentStr)
		err := os.WriteFile("server_storage/"+filename, content, 0644)
		if err != nil {
			log.Printf("[ERROR] Writing file %s: %v\n", filename, err)
		} else {
			log.Printf("[INFO] File '%s' uploaded successfully\n", filename)
			fileStateMu.Lock()
			fileState[filename] = time.Now()
			fileStateMu.Unlock()
		}

	case "delete":
		err := os.Remove("server_storage/" + filename)
		if err != nil {
			log.Printf("[ERROR] Deleting file %s: %v\n", filename, err)
		} else {
			log.Printf("[INFO] File '%s' deleted successfully\n", filename)
			fileStateMu.Lock()
			delete(fileState, filename)
			fileStateMu.Unlock()
		}

	case "request":
		content, err := os.ReadFile("server_storage/" + filename)
		if err != nil {
			log.Printf("[ERROR] Reading file %s: %v\n", filename, err)
			return
		}
		clientsMu.Lock()
		for client := range clients {
			client.WriteJSON(map[string]interface{}{
				"operation": "content",
				"filename":  filename,
				"content":   string(content),
			})
		}
		clientsMu.Unlock()
		log.Printf("[INFO] Sent content of file '%s' to all connected clients\n", filename)

	default:
		log.Printf("[ERROR] Unknown operation '%s' in message: %v\n", operation, msg)
	}
}

func broadcastFileChanges() {
	for {
		time.Sleep(1 * time.Second)
		fileStateMu.Lock()
		currentFileState := make(map[string]time.Time)
		for filename, modTime := range fileState {
			currentFileState[filename] = modTime
		}
		fileStateMu.Unlock()

		clientsMu.Lock()
		for client := range clients {
			for filename, modTime := range currentFileState {
				err := client.WriteJSON(map[string]interface{}{
					"filename":  filename,
					"operation": "update",
					"timestamp": modTime.UnixNano(),
				})
				if err != nil {
					log.Printf("[ERROR] Broadcasting file update for '%s' failed: %v\n", filename, err)
					delete(clients, client)
				}
			}
		}
		clientsMu.Unlock()

		log.Println("[INFO] Broadcasted file changes to all connected clients")
	}
}
