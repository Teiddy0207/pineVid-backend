package response

import "time"

// CommentUser is the nested author sub-object within CommentResponse
// (mirrors the VideoCreator convention used by response.VideoResponse).
type CommentUser struct {
	ID     string `json:"id"     example:"550e8400-e29b-41d4-a716-446655440000"`
	Name   string `json:"name"   example:"quanh_dep_trai"`
	Avatar string `json:"avatar" example:"http://localhost:9000/raw-videos/avatars/user.jpg"`
}

type CommentResponse struct {
	ID        string      `json:"id"`
	VideoID   string      `json:"video_id"`
	User      CommentUser `json:"user"`
	Content   string      `json:"content"`
	CreatedAt time.Time   `json:"created_at"`
}
