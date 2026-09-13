package events

import "sync"

// ChatMessage represents a single event broadcast into a live stream room.
// Type discriminates what kind of event this is:
//   - "chat" (default, zero value): a regular chat message — Text is set.
//   - "heart": someone sent a heart reaction — Value carries the room's new
//     running total, straight from the real HeartStream API call.
//
// Kept as one struct (rather than a per-type union) so the existing
// single-channel ChatHub plumbing doesn't need to change shape.
type ChatMessage struct {
	StreamID  string `json:"stream_id"`
	Type      string `json:"type,omitempty"`
	Username  string `json:"username,omitempty"`
	Avatar    string `json:"avatar,omitempty"`
	Text      string `json:"text,omitempty"`
	Value     int64  `json:"value,omitempty"`
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
