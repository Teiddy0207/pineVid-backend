package entity

import (
	"errors"
	"time"
)

var (
	ErrPostNotFound  = errors.New("post not found")
	ErrPostForbidden = errors.New("you do not have permission to modify this post")
)

type Post struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	UserName   string    `json:"user_name"`
	UserAvatar string    `json:"user_avatar"`
	Content    string    `json:"content"`
	ImageURL   string    `json:"image_url"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
