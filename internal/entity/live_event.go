package entity

import (
	"errors"
	"time"
)

// Live event types carried on the Redis Stream (field "type").
const (
	LiveEventChat  = "chat"
	LiveEventHeart = "heart"
)

// ErrLiveEventsUnavailable means the event could not be appended to the
// Redis Stream; the request fails instead of the event being lost silently.
var ErrLiveEventsUnavailable = errors.New("live events log unavailable")

// LiveEvent is one chat line or heart sent into a live room. The backend
// appends it to the stream; the live-agg worker resolves the broadcast
// session from At and persists/aggregates it. Field names on the wire are
// documented in docs/message-contract.md.
type LiveEvent struct {
	Type         string
	LivestreamID string
	UserID       string
	Username     string
	Avatar       string
	Text         string
	At           time.Time
}

// LiveSessionStats are the per-broadcast totals maintained by live-agg.
type LiveSessionStats struct {
	Hearts         int64 `json:"hearts"`
	ChatMessages   int64 `json:"chat_messages"`
	UniqueChatters int   `json:"unique_chatters"`
}
