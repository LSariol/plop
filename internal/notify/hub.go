// internal/notify/hub.go

package notify

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WSMessage is the envelope for all WebSocket messages.
type WSMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Client represents a single connected desktop client.
type Client struct {
	id     string
	userID string
	conn   *websocket.Conn
	send   chan WSMessage
}

// Hub manages all connected clients.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]*Client
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]*Client),
	}
}

// Register adds a new client to the hub.
func (h *Hub) Register(id, userID string, conn *websocket.Conn) *Client {
	c := &Client{
		id:     id,
		userID: userID,
		conn:   conn,
		send:   make(chan WSMessage, 32), // buffered so Broadcast doesn't block
	}
	h.mu.Lock()
	h.clients[id] = c
	h.mu.Unlock()

	go c.writePump()
	return c
}

// Unregister removes a client from the hub and closes its send channel.
func (h *Hub) Unregister(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c, ok := h.clients[id]; ok {
		close(c.send)
		delete(h.clients, id)
	}
}

// Broadcast sends a message to all connected clients.
func (h *Hub) Broadcast(msg WSMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.clients {
		select {
		case c.send <- msg:
		default:
			// Channel full — client is too slow or stuck. Log and skip.
			log.Printf("warn: send buffer full for client %s, dropping message", c.id)
		}
	}
}

// SendToUser delivers a message to all connected desktops belonging to userID.
func (h *Hub) SendToUser(userID string, msg WSMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.clients {
		if c.userID != userID {
			continue
		}
		select {
		case c.send <- msg:
		default:
			log.Printf("warn: send buffer full for client %s, dropping message", c.id)
		}
	}
}

// Send delivers a message to a single specific client.
// Used when replying to a hello with that client's pending payload IDs.
func (h *Hub) Send(clientID string, msg WSMessage) {
	h.mu.RLock()
	c, ok := h.clients[clientID]
	h.mu.RUnlock()
	if !ok {
		return
	}
	select {
	case c.send <- msg:
	default:
		log.Printf("warn: send buffer full for client %s, dropping message", clientID)
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteJSON(msg); err != nil {
				log.Printf("write to client %s: %v", c.id, err)
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
