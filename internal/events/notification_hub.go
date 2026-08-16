package events

import (
	"sync"

	"github.com/evrone/go-clean-template/internal/entity"
)

type NotificationHub struct {
	mu          sync.RWMutex
	subscribers map[string][]chan entity.Notification // userID -> list of subscriber channels
}

func NewNotificationHub() *NotificationHub {
	return &NotificationHub{
		subscribers: make(map[string][]chan entity.Notification),
	}
}

// Subscribe returns a channel that receives real-time notifications for a specific userID
func (h *NotificationHub) Subscribe(userID string) (<-chan entity.Notification, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan entity.Notification, 16)
	h.subscribers[userID] = append(h.subscribers[userID], ch)

	unsubscribe := func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		subs := h.subscribers[userID]
		for i, sub := range subs {
			if sub == ch {
				h.subscribers[userID] = append(subs[:i], subs[i+1:]...)
				close(ch)
				break
			}
		}
		if len(h.subscribers[userID]) == 0 {
			delete(h.subscribers, userID)
		}
	}

	return ch, unsubscribe
}

// SendDirect sends a notification to a specific online user
func (h *NotificationHub) SendDirect(userID string, notif entity.Notification) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	subs, exists := h.subscribers[userID]
	if !exists {
		return
	}

	for _, ch := range subs {
		select {
		case ch <- notif:
		default:
			// Buffer full, skip to prevent blocking
		}
	}
}
