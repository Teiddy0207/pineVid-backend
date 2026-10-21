package entity

import (
	"errors"
	"time"
)

var ErrLiveSessionNotFound = errors.New("live session not found")

// LiveSession represents a single broadcast event on a channel. Each time a
// streamer goes live a new LiveSession is created; the parent Livestream row
// holds only the stream-key config and channel identity.
//
// Key lifecycle fields:
//   - StartedAt: set when the RTMP publish is authenticated and is_live flips true.
//   - EndedAt:   set when the grace period elapses with no reconnect (or when the
//     streamer ends the stream explicitly from Studio).
//   - ReplayVideoID: set by processDVR once the transcode job is queued.
type LiveSession struct {
	ID            string     `json:"id"`
	LivestreamID  string     `json:"livestream_id"`
	Title         string     `json:"title"`
	ThumbnailURL  string     `json:"thumbnail_url"`
	Category      string     `json:"category"`
	StartedAt     time.Time  `json:"started_at"`
	EndedAt       *time.Time `json:"ended_at,omitempty"`
	ReplayVideoID string     `json:"replay_video_id,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	// Stats are filled from live_session_stats (zero until live-agg has
	// processed the session's first event).
	Stats LiveSessionStats `json:"stats"`
}

// LiveChatMessage is a single persisted chat line within a live session.
// It mirrors what the ChatHub broadcasts in real-time but is durable —
// the SSE path never reads from here; this is only used for replay.
type LiveChatMessage struct {
	ID        int64     `json:"id"`
	SessionID string    `json:"session_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Avatar    string    `json:"avatar"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}
