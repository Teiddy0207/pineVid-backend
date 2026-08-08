package entity

import (
	"errors"
	"time"
)

var (
	ErrVideoNotFound = errors.New("video not found")
)

type VideoStatus string

const (
	VideoStatusPending    VideoStatus = "pending"
	VideoStatusProcessing VideoStatus = "processing"
	VideoStatusComplete   VideoStatus = "complete"
	VideoStatusFailed     VideoStatus = "failed"
)

type VideoVisibility string

const (
	VideoVisibilityPublic   VideoVisibility = "public"
	VideoVisibilityPrivate  VideoVisibility = "private"
	VideoVisibilityUnlisted VideoVisibility = "unlisted"
)

type Video struct {
	ID           string          `json:"id"`
	UserID       string          `json:"user_id"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	Category     string          `json:"category"`
	Status       VideoStatus     `json:"status"`
	Visibility   VideoVisibility `json:"visibility"`
	RawS3Key     string          `json:"raw_s3_key"`
	HLSUrl       string          `json:"hls_url"`
	ThumbnailUrl string          `json:"thumbnail_url"`
	Duration     string          `json:"duration"`
	Views        int64           `json:"views"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}
