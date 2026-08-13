package response

// UserResponse is the DTO returned to clients for all user-related endpoints.
// It never exposes the password hash.
type UserResponse struct {
	ID        string `json:"id"         example:"550e8400-e29b-41d4-a716-446655440000"`
	Username  string `json:"username"   example:"quanh_dep_trai"`
	Email     string `json:"email"      example:"whoami20945@gmail.com"`
	Avatar    string `json:"avatar"     example:"http://localhost:9000/raw-videos/avatars/user.jpg"`
	CreatedAt string `json:"created_at" example:"2026-01-01T00:00:00Z"`
} // @name response.UserResponse

type ChannelDetailsResponse struct {
	ID               string `json:"id"`
	Username         string `json:"username"`
	Email            string `json:"email"`
	Avatar           string `json:"avatar"`
	Bio              string `json:"bio"`
	SubscribersCount int64  `json:"subscribers_count"`
	TotalVideos      int    `json:"total_videos"`
	TotalViews       int64  `json:"total_views"`
	CreatedAt        string `json:"created_at"`
} // @name response.ChannelDetailsResponse
