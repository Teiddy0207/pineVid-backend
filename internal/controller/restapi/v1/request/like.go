package request

type LikeVideoRequest struct {
	VideoID string `json:"video_id" validate:"required"`
}

type HeartStreamRequest struct {
	StreamID string `json:"stream_id" validate:"required"`
}
