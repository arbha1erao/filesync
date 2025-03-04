package server

import (
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Server struct {
	clients     map[*websocket.Conn]bool
	clientsMu   sync.Mutex
	fileState   map[string]time.Time
	fileStateMu sync.Mutex
}

func NewServer() *Server {
	return &Server{
		clients:   make(map[*websocket.Conn]bool),
		fileState: make(map[string]time.Time),
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func (s *Server) HandleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ERROR] WebSocket upgrade error: %v, from client: %s", err, r.RemoteAddr)
		return
	}
	defer ws.Close()

	s.clientsMu.Lock()
	s.clients[ws] = true
	s.clientsMu.Unlock()

	log.Printf("[INFO] New client connected: %s", r.RemoteAddr)

	s.SendInitialFiles(ws)

	for {
		var msg map[string]interface{}
		if err := ws.ReadJSON(&msg); err != nil {
			log.Printf("[ERROR] WebSocket read error: %v, from client: %s", err, r.RemoteAddr)
			s.clientsMu.Lock()
			delete(s.clients, ws)
			s.clientsMu.Unlock()
			break
		}
		log.Printf("[INFO] Received message from client %s: %v", r.RemoteAddr, msg)
		s.HandleMessage(msg)
	}
}

func (s *Server) SendInitialFiles(ws *websocket.Conn) {
	files, err := os.ReadDir("server_storage")
	if err != nil {
		log.Printf("[ERROR] Reading server storage directory: %v", err)
		return
	}

	for _, f := range files {
		content, err := os.ReadFile("server_storage/" + f.Name())
		if err != nil {
			log.Printf("[ERROR] Reading file %s: %v", f.Name(), err)
			continue
		}

		err = ws.WriteJSON(map[string]interface{}{
			"filename": f.Name(),
			"content":  string(content),
		})
		if err != nil {
			log.Printf("[ERROR] Writing JSON to WebSocket: %v", err)
			return
		}
	}
}

func (s *Server) HandleMessage(msg map[string]interface{}) {
	filename, ok := msg["filename"].(string)
	if !ok || filename == "" {
		log.Printf("[ERROR] Invalid filename in message: %v", msg)
		return
	}

	operation, ok := msg["operation"].(string)
	if !ok || operation == "" {
		log.Printf("[ERROR] Invalid operation in message: %v", msg)
		return
	}

	log.Printf("[INFO] Handling operation '%s' for file '%s'", operation, filename)

	s.fileStateMu.Lock()
	defer s.fileStateMu.Unlock()

	switch operation {
	case "upload":
		contentStr, ok := msg["content"].(string)
		if !ok {
			log.Printf("[ERROR] Invalid content field in upload message: %v", msg)
			return
		}
		err := os.WriteFile("server_storage/"+filename, []byte(contentStr), 0644)
		if err != nil {
			log.Printf("[ERROR] Writing file %s: %v", filename, err)
		} else {
			s.fileState[filename] = time.Now()
			log.Printf("[INFO] File '%s' uploaded successfully", filename)
		}

	case "delete":
		err := os.Remove("server_storage/" + filename)
		if err != nil {
			log.Printf("[ERROR] Deleting file %s: %v", filename, err)
		} else {
			delete(s.fileState, filename)
			log.Printf("[INFO] File '%s' deleted successfully", filename)
		}

	case "request":
		content, err := os.ReadFile("server_storage/" + filename)
		if err != nil {
			log.Printf("[ERROR] Reading file %s: %v", filename, err)
			return
		}
		s.Broadcast(map[string]interface{}{
			"operation": "content",
			"filename":  filename,
			"content":   string(content),
		})
	default:
		log.Printf("[ERROR] Unknown operation '%s' in message: %v", operation, msg)
	}
}

func (s *Server) Broadcast(message map[string]interface{}) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	for client := range s.clients {
		if err := client.WriteJSON(message); err != nil {
			log.Printf("[ERROR] Broadcasting message failed: %v", err)
			delete(s.clients, client)
		}
	}
}

func (s *Server) BroadcastFileChanges() {
	for {
		time.Sleep(1 * time.Second)
		currentFileState := make(map[string]time.Time)

		s.fileStateMu.Lock()
		for filename, modTime := range s.fileState {
			currentFileState[filename] = modTime
		}
		s.fileStateMu.Unlock()

		for filename, modTime := range currentFileState {
			s.Broadcast(map[string]interface{}{
				"filename":  filename,
				"operation": "update",
				"timestamp": modTime.UnixNano(),
			})
		}
	}
}
