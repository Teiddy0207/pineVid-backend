package entity

import "time"

// WatchHistoryEntry records the last time a user watched a given video,
// and how many seconds of it they watched (upserted, one row per user+video).
type WatchHistoryEntry struct {
	UserID        string    `json:"user_id"`
	VideoID       string    `json:"video_id"`
	WatchSeconds  int       `json:"watch_seconds"`
	LastWatchedAt time.Time `json:"last_watched_at"`
}
