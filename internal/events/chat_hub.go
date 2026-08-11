package events

import "sync"

// ChatMessage represents a single chat message in a live stream room
type ChatMessage struct {
	StreamID  string `json:"stream_id"`
	Username  string `json:"username"`
	Avatar    string `json:"avatar"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}

// ChatHub manages room-based SSE channels for live stream chats
type ChatHub struct {
	mu sync.RWMutex
	// rooms maps streamID -> map of client channels
	rooms map[string]map[chan ChatMessage]struct{}
}

func NewChatHub() *ChatHub {
	return &ChatHub{
		rooms: make(map[string]map[chan ChatMessage]struct{}),
	}
}

// Subscribe adds a client channel to a specific live stream room
func (h *ChatHub) Subscribe(streamID string) (<-chan ChatMessage, func()) {
	ch := make(chan ChatMessage, 20)
	h.mu.Lock()
	if _, exists := h.rooms[streamID]; !exists {
		h.rooms[streamID] = make(map[chan ChatMessage]struct{})
	}
	h.rooms[streamID][ch] = struct{}{}
	h.mu.Unlock()

	unsubscribe := func() {
		h.mu.Lock()
		if room, exists := h.rooms[streamID]; exists {
			delete(room, ch)
			if len(room) == 0 {
				delete(h.rooms, streamID)
			}
		}
		close(ch)
		h.mu.Unlock()
	}

	return ch, unsubscribe
}

func safeSend(ch chan ChatMessage, msg ChatMessage) {
	defer func() {
		_ = recover()
	}()
	select {
	case ch <- msg:
	default:
	}
}

// Broadcast sends a ChatMessage to all subscribers in a live stream room
func (h *ChatHub) Broadcast(streamID string, msg ChatMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if room, exists := h.rooms[streamID]; exists {
		for ch := range room {
			safeSend(ch, msg)
		}
	}
}
