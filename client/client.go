package client

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

type Client struct {
	Conn        *websocket.Conn
	Watcher     *fsnotify.Watcher
	IgnoreFiles map[string]bool
	mu          sync.Mutex
}

func NewClient() (*Client, error) {
	if err := os.MkdirAll(syncDir, 0755); err != nil {
		return nil, err
	}

	conn, _, err := websocket.DefaultDialer.Dial(serverURL, nil)
	if err != nil {
		return nil, err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := watcher.Add(syncDir); err != nil {
		return nil, err
	}

	return &Client{
		Conn:        conn,
		Watcher:     watcher,
		IgnoreFiles: make(map[string]bool),
	}, nil
}

func (c *Client) HandleLocalChange(path string) {
	filename := filepath.Base(path)

	c.mu.Lock()
	if c.IgnoreFiles[filename] {
		log.Printf("[DEBUG] Ignoring local change for file: %s", filename)
		delete(c.IgnoreFiles, filename)
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()

	content, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[ERROR] Reading file %s: %v", path, err)
		return
	}

	log.Printf("[INFO] Sending upload request for file: %s", filename)
	err = c.Conn.WriteJSON(map[string]interface{}{
		"operation": "upload",
		"filename":  filename,
		"content":   string(content),
	})
	if err != nil {
		log.Printf("[ERROR] Failed to send upload message for file %s: %v", filename, err)
	}
}

func (c *Client) HandleLocalDelete(filename string) {
	log.Printf("[INFO] Sending delete message for file: %s", filename)
	err := c.Conn.WriteJSON(map[string]interface{}{
		"operation": "delete",
		"filename":  filename,
	})
	if err != nil {
		log.Printf("[ERROR] Failed to send delete message for file %s: %v", filename, err)
	}
}

func (c *Client) HandleServerMessage(msg map[string]interface{}) {
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
			log.Printf("[INFO] Requesting file content for: %s", filename)
			err = c.Conn.WriteJSON(map[string]interface{}{
				"operation": "request",
				"filename":  filename,
			})
			if err != nil {
				log.Printf("[ERROR] Failed to send request for file %s: %v", filename, err)
			}
		}
	case "content":
		content, ok := msg["content"].(string)
		if !ok {
			log.Println("[ERROR] Invalid server message format: missing content.")
			return
		}

		c.mu.Lock()
		c.IgnoreFiles[filename] = true
		c.mu.Unlock()

		err := os.WriteFile(syncDir+"/"+filename, []byte(content), 0644)
		if err != nil {
			log.Printf("[ERROR] Failed to write file %s: %v", filename, err)
		} else {
			log.Printf("[INFO] Updated file from server: %s", filename)
		}
	}
}

func (c *Client) WatchFiles() {
	for event := range c.Watcher.Events {
		if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
			log.Printf("[INFO] File change detected: %s", event.Name)
			c.HandleLocalChange(event.Name)
		}
		if event.Op&fsnotify.Remove == fsnotify.Remove {
			log.Printf("[INFO] File deletion detected: %s", event.Name)
			c.HandleLocalDelete(filepath.Base(event.Name))
		}
	}
}

func (c *Client) ListenToServer() {
	for {
		var msg map[string]interface{}
		if err := c.Conn.ReadJSON(&msg); err != nil {
			log.Printf("[ERROR] Error reading message from server: %v", err)
			return
		}
		c.HandleServerMessage(msg)
	}
}
