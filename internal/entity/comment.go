package entity

import (
	"errors"
	"time"
)

var (
	ErrCommentNotFound  = errors.New("comment not found")
	ErrInvalidParentID  = errors.New("parent comment does not exist or belongs to a different video")
)

type Comment struct {
	ID         string    `json:"id"`
	VideoID    string    `json:"video_id"`
	UserID     string    `json:"user_id"`
	UserName   string    `json:"user_name"`
	UserAvatar string    `json:"user_avatar"`
	Content    string    `json:"content"`
	ParentID   *string   `json:"parent_id,omitempty"`
	ReplyCount int       `json:"reply_count"`
	CreatedAt  time.Time `json:"created_at"`
}
