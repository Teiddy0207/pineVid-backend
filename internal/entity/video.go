package entity

import (
	"errors"
	"time"
)

var (
	ErrVideoNotFound          = errors.New("video not found")
	ErrUnsupportedVideoFormat = errors.New("unsupported video format: only .mp4, .mov, .mkv are allowed")
	ErrVideoForbidden         = errors.New("you do not have permission to modify this video")
	ErrVideoNotFailed         = errors.New("video transcode has not failed, nothing to retry")
)

// AllowedVideoExtensions is the upload allow-list per BUSINESS_REQUIREMENTS.md
// (".mp4", ".mov", ".mkv").
var AllowedVideoExtensions = map[string]bool{
	".mp4": true,
	".mov": true,
	".mkv": true,
}

type VideoStatus string

const (
	VideoStatusPending    VideoStatus = "pending"
	VideoStatusProcessing VideoStatus = "processing"
	VideoStatusComplete   VideoStatus = "complete"
	VideoStatusFailed     VideoStatus = "failed"
	// VideoStatusBanned is set by an admin takedown. Every public listing
	// already filters on status = complete, so a banned video disappears
	// everywhere; GetByID hides it from everyone except its owner.
	VideoStatusBanned VideoStatus = "banned"
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
	UserName     string          `json:"user_name"`
	UserAvatar   string          `json:"user_avatar"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	Category     string          `json:"category"`
	Status       VideoStatus     `json:"status"`
	Visibility   VideoVisibility `json:"visibility"`
	RawS3Key     string          `json:"raw_s3_key"`
	HLSUrl       string          `json:"hls_url"`
	ThumbnailUrl string          `json:"thumbnail_url"`
	Duration     string          `json:"duration"`
	IsReel       bool            `json:"is_reel"`
	Views        int64           `json:"views"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// ViewableBy reports whether userID ("" = anonymous) may watch this video.
// The owner can always see their own video (the Studio shows its state);
// everyone else only sees finished, non-banned, non-private videos.
// Unlisted videos are viewable by anyone who has the link, like YouTube.
func (v Video) ViewableBy(userID string) bool {
	if userID != "" && v.UserID == userID {
		return true
	}
	return v.Status == VideoStatusComplete && v.Visibility != VideoVisibilityPrivate
}
