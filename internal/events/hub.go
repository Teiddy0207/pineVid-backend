package events

import "sync"

// VideoEvent represents a video status change event
type VideoEvent struct {
	VideoID string `json:"video_id"`
	Status  string `json:"status"`
	HLSUrl  string `json:"hls_url,omitempty"`
}

// Hub is a simple broadcast hub for SSE clients
type Hub struct {
	mu      sync.RWMutex
	clients map[chan VideoEvent]struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[chan VideoEvent]struct{}),
	}
}

// Subscribe adds a new SSE client and returns the channel + unsubscribe func
func (h *Hub) Subscribe() (<-chan VideoEvent, func()) {
	ch := make(chan VideoEvent, 10)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()

	unsubscribe := func() {
		h.mu.Lock()
		delete(h.clients, ch)
		close(ch)
		h.mu.Unlock()
	}
	return ch, unsubscribe
}

// Broadcast sends an event to all connected SSE clients
func (h *Hub) Broadcast(event VideoEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch := range h.clients {
		select {
		case ch <- event:
		default:
			// skip slow clients
		}
	}
}
