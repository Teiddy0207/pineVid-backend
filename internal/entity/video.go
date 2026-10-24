package entity

import (
	"errors"
	"time"
)

var (
	ErrVideoNotFound          = errors.New("video not found")
	ErrUnsupportedVideoFormat = errors.New("unsupported video format: only .mp4, .mov, .mkv are allowed")
	ErrUnsupportedImageFormat = errors.New("unsupported image format: only .jpg, .jpeg, .png, .webp, .gif are allowed")
	ErrVideoForbidden         = errors.New("you do not have permission to modify this video")
	ErrVideoNotFailed         = errors.New("video transcode has not failed, nothing to retry")
	ErrViewTicketInvalid      = errors.New("view ticket is invalid, expired or already used")
	ErrViewTicketTooEarly     = errors.New("minimum watch time not reached yet")
)

// MinWatchForView is how long a viewer must watch before a view counts
// (BUSINESS_REQUIREMENTS.md §3.1: "xem liên tục trên 10 giây").
const MinWatchForView = 10 * time.Second

// AllowedVideoExtensions is the upload allow-list per BUSINESS_REQUIREMENTS.md
// (".mp4", ".mov", ".mkv").
var AllowedVideoExtensions = map[string]bool{
	".mp4": true,
	".mov": true,
	".mkv": true,
}

// AllowedImageExtensions is the upload allow-list for avatars, thumbnails and
// post images.
var AllowedImageExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
	".gif":  true,
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
	ReplaySessionID string          `json:"replay_session_id,omitempty"`
	ReplayStartedAt *time.Time      `json:"replay_started_at,omitempty"`
	ID              string          `json:"id"`
	UserID          string          `json:"user_id"`
	UserName        string          `json:"user_name"`
	UserAvatar      string          `json:"user_avatar"`
	Title           string          `json:"title"`
	Description     string          `json:"description"`
	Category        string          `json:"category"`
	Status          VideoStatus     `json:"status"`
	Visibility      VideoVisibility `json:"visibility"`
	RawS3Key        string          `json:"raw_s3_key"`
	HLSUrl          string          `json:"hls_url"`
	ThumbnailUrl    string          `json:"thumbnail_url"`
	Duration        string          `json:"duration"`
	IsReel          bool            `json:"is_reel"`
	Views           int64           `json:"views"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`

	// Transcode job tracking (docs/message-contract.md §4.3). QueuedAt →
	// TranscodeCompletedAt is the upload-to-playable latency (criterion T2).
	CurrentJobID         string     `json:"current_job_id,omitempty"`
	QueuedAt             *time.Time `json:"queued_at,omitempty"`
	TranscodeStartedAt   *time.Time `json:"transcode_started_at,omitempty"`
	TranscodeCompletedAt *time.Time `json:"transcode_completed_at,omitempty"`
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
