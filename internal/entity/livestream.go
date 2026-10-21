package entity

import (
	"errors"
	"time"
)

var (
	ErrLivestreamNotFound = errors.New("livestream not found")
	ErrInvalidStreamKey   = errors.New("invalid stream key")
	ErrStreamBanned       = errors.New("this stream has been banned")
	ErrEmptyChatMessage   = errors.New("text cannot be empty")
	ErrChatMessageTooLong = errors.New("chat message is too long (max 500 characters)")
)

type Livestream struct {
	ID           string `json:"id"`
	UserID       string `json:"user_id"`
	UserName     string `json:"user_name"`
	UserAvatar   string `json:"user_avatar"`
	StreamKey    string `json:"stream_key"`
	Title        string `json:"title"`
	Category     string `json:"category"`
	ThumbnailURL string `json:"thumbnail_url"`
	IsLive       bool   `json:"is_live"`
	HLSUrl       string `json:"hls_url"`
	ViewersCount int64  `json:"viewers_count"`
	// IsBanned is an admin takedown of this channel's stream: on_publish
	// rejects the key until an admin unbans it.
	IsBanned bool `json:"is_banned"`
	// OwnerBanned mirrors users.is_banned of the streamer (read-only join).
	OwnerBanned bool       `json:"-"`
	StartedAt   *time.Time `json:"started_at"`
	EndedAt     *time.Time `json:"ended_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
