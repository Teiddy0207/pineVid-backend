package entity

import (
	"errors"
	"time"
)

var (
	ErrLivestreamNotFound = errors.New("livestream not found")
	ErrInvalidStreamKey   = errors.New("invalid stream key")
)

type Livestream struct {
	ID           string     `json:"id"`
	UserID       string     `json:"user_id"`
	StreamKey    string     `json:"stream_key"`
	Title        string     `json:"title"`
	Category     string     `json:"category"`
	IsLive       bool       `json:"is_live"`
	HLSUrl       string     `json:"hls_url"`
	ViewersCount int64      `json:"viewers_count"`
	StartedAt    *time.Time `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
