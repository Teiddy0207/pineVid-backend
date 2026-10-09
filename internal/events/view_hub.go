package events

import (
	"context"
	"sync"
	"time"
)

// ViewUpdate is what a watch page receives over WebSocket: the video's total
// views and the server time (Unix ms) of the view that produced it, which
// criterion V3 (≤ 2 s to the browser) is measured against.
type ViewUpdate struct {
	VideoID    string `json:"video_id"`
	TotalViews int64  `json:"total_views"`
	TS         int64  `json:"ts"`
}

// ViewHub fans video.views.updated out to the WebSocket clients watching each
// video. Updates are coalesced: at most one send per video per flush
// interval, carrying the highest total seen — under 1 000 views/s a browser
// gets a few frames per second instead of a thousand.
type ViewHub struct {
	interval time.Duration

	mu     sync.Mutex
	subs   map[string]map[chan ViewUpdate]struct{}
	latest map[string]ViewUpdate
	dirty  map[string]struct{}
}

func NewViewHub(interval time.Duration) *ViewHub {
	return &ViewHub{
		interval: interval,
		subs:     make(map[string]map[chan ViewUpdate]struct{}),
		latest:   make(map[string]ViewUpdate),
		dirty:    make(map[string]struct{}),
	}
}

// Subscribe registers a client for videoID. The channel holds only the newest
// update; the returned func unsubscribes and closes it.
func (h *ViewHub) Subscribe(videoID string) (<-chan ViewUpdate, func()) {
	ch := make(chan ViewUpdate, 1)

	h.mu.Lock()
	if h.subs[videoID] == nil {
		h.subs[videoID] = make(map[chan ViewUpdate]struct{})
	}
	h.subs[videoID][ch] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	return ch, func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.subs[videoID], ch)
			if len(h.subs[videoID]) == 0 {
				delete(h.subs, videoID)
				delete(h.latest, videoID)
				delete(h.dirty, videoID)
			}
			close(ch)
			h.mu.Unlock()
		})
	}
}

// Publish records an update; it is delivered on the next flush. Totals only
// grow, so an update that arrives late with a smaller total is dropped.
func (h *ViewHub) Publish(u ViewUpdate) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, watched := h.subs[u.VideoID]; !watched {
		return
	}
	if prev, ok := h.latest[u.VideoID]; ok && u.TotalViews <= prev.TotalViews {
		return
	}
	h.latest[u.VideoID] = u
	h.dirty[u.VideoID] = struct{}{}
}

// Flush sends the pending update of every changed video to its clients.
func (h *ViewHub) Flush() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for videoID := range h.dirty {
		u := h.latest[videoID]
		for ch := range h.subs[videoID] {
			// Replace a not-yet-read older update with the newest one.
			select {
			case <-ch:
			default:
			}
			ch <- u
		}
		delete(h.dirty, videoID)
	}
}

// Run flushes every interval until ctx is done.
func (h *ViewHub) Run(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.Flush()
		}
	}
}
