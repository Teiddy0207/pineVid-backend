package response

import "time"

// PostAuthor is the nested author sub-object within PostResponse (mirrors
// the VideoCreator/CommentUser convention).
type PostAuthor struct {
	ID     string `json:"id"     example:"550e8400-e29b-41d4-a716-446655440000"`
	Name   string `json:"name"   example:"quanh_dep_trai"`
	Avatar string `json:"avatar" example:"http://localhost:9000/raw-videos/avatars/user.jpg"`
}

type PostResponse struct {
	ID        string     `json:"id"`
	Author    PostAuthor `json:"author"`
	Content   string     `json:"content"`
	ImageURL  string     `json:"image_url,omitempty"`
	LikeCount int64      `json:"like_count"`
	IsLiked   bool       `json:"is_liked"`
	CreatedAt time.Time  `json:"created_at"`
}

// PostLikeResponse is the toggle-like result, mirroring CommentLikeResponse.
type PostLikeResponse struct {
	PostID     string `json:"post_id"`
	Liked      bool   `json:"liked"`
	TotalLikes int64  `json:"total_likes"`
}
