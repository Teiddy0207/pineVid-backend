package events

import "sync"

// VideoEvent represents a video status change event
type VideoEvent struct {
	// UserID is the video owner; the hub only delivers the event to that
	// user's subscriptions. Not serialized.
	UserID  string `json:"-"`
	VideoID string `json:"video_id"`
	Status  string `json:"status"`
	HLSUrl  string `json:"hls_url,omitempty"`
}

// Hub is a simple broadcast hub for SSE clients. Each subscription belongs
// to one user and only receives events about that user's videos.
type Hub struct {
	mu      sync.RWMutex
	clients map[chan VideoEvent]string // channel -> subscriber userID
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[chan VideoEvent]string),
	}
}

// Subscribe adds a new SSE client for userID and returns the channel + unsubscribe func
func (h *Hub) Subscribe(userID string) (<-chan VideoEvent, func()) {
	ch := make(chan VideoEvent, 10)
	h.mu.Lock()
	h.clients[ch] = userID
	h.mu.Unlock()

	unsubscribe := func() {
		h.mu.Lock()
		delete(h.clients, ch)
		close(ch)
		h.mu.Unlock()
	}
	return ch, unsubscribe
}

// Broadcast sends an event to every connected SSE client of the video's owner
func (h *Hub) Broadcast(event VideoEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch, userID := range h.clients {
		if userID != event.UserID {
			continue
		}
		select {
		case ch <- event:
		default:
			// skip slow clients
		}
	}
}
