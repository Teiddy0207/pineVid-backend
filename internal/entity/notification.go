package entity

import "time"

type NotificationType string

const (
	NotificationTypeLiveStart NotificationType = "live_start"
	NotificationTypeNewVideo  NotificationType = "new_video"
)

type Notification struct {
	ID           string           `json:"id"`
	UserID       string           `json:"user_id"`
	SenderID     string           `json:"sender_id"`
	SenderName   string           `json:"sender_name"`
	SenderAvatar string           `json:"sender_avatar"`
	Type         NotificationType `json:"type"`
	Title        string           `json:"title"`
	Message      string           `json:"message"`
	TargetURL    string           `json:"target_url"`
	IsRead       bool             `json:"is_read"`
	CreatedAt    time.Time        `json:"created_at"`
}
