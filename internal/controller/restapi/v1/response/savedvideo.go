package response

type SaveVideoResponse struct {
	VideoID string `json:"video_id"`
	Saved   bool   `json:"saved"`
}
