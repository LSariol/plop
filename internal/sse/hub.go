package sse

import "sync"

// Hub routes server-sent events to subscribed browser sessions by user ID.
type Hub struct {
	mu      sync.RWMutex
	clients map[string][]chan string // userID → open subscriber channels
}

func NewHub() *Hub {
	return &Hub{clients: make(map[string][]chan string)}
}

// Subscribe registers a listener for the given user. The returned channel
// receives JSON event data strings. Call unsubscribe when the client disconnects.
func (h *Hub) Subscribe(userID string) (events <-chan string, unsubscribe func()) {
	ch := make(chan string, 8)
	h.mu.Lock()
	h.clients[userID] = append(h.clients[userID], ch)
	h.mu.Unlock()

	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		list := h.clients[userID]
		for i, c := range list {
			if c == ch {
				h.clients[userID] = append(list[:i], list[i+1:]...)
				break
			}
		}
		if len(h.clients[userID]) == 0 {
			delete(h.clients, userID)
		}
		close(ch)
	}
}

// Publish sends a JSON data string to all active subscribers for userID.
func (h *Hub) Publish(userID, data string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, ch := range h.clients[userID] {
		select {
		case ch <- data:
		default:
			// Client too slow — drop the event rather than blocking.
		}
	}
}
