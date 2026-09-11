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
	ID         string      `json:"id"`
	VideoID    string      `json:"video_id"`
	User       CommentUser `json:"user"`
	Content    string      `json:"content"`
	ParentID   *string     `json:"parent_id,omitempty"`
	ReplyCount int         `json:"reply_count"`
	LikeCount  int64       `json:"like_count"`
	IsLiked    bool        `json:"is_liked"`
	CreatedAt  time.Time   `json:"created_at"`
}

// CommentLikeResponse is the toggle-like result, mirroring LikeResponse.
type CommentLikeResponse struct {
	CommentID  string `json:"comment_id"`
	Liked      bool   `json:"liked"`
	TotalLikes int64  `json:"total_likes"`
}

// CommentPageResponse is like PageResponse[CommentResponse] but also carries
// TotalAllCount — the true comment count including replies, which the
// paginated Pagination.TotalItems deliberately excludes (that field counts
// only top-level comments, matching what's actually paginated).
type CommentPageResponse struct {
	Success       bool              `json:"success"`
	Data          []CommentResponse `json:"data"`
	Pagination    PaginationMeta    `json:"pagination"`
	TotalAllCount int               `json:"total_all_count"`
}
