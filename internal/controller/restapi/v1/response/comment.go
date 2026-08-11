package response

import "time"

type CommentResponse struct {
	ID         string    `json:"id"`
	VideoID    string    `json:"video_id"`
	UserID     string    `json:"user_id"`
	UserName   string    `json:"user_name"`
	UserAvatar string    `json:"user_avatar"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}
