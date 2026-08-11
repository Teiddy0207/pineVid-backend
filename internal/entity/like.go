package entity

import "time"

type VideoLike struct {
	ID        string    `json:"id"`
	VideoID   string    `json:"video_id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}
