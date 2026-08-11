package response

type LikeResponse struct {
	VideoID    string `json:"video_id"`
	Liked      bool   `json:"liked"`
	TotalLikes int64  `json:"total_likes"`
}

type HeartResponse struct {
	StreamID    string `json:"stream_id"`
	TotalHearts int64  `json:"total_hearts"`
}
