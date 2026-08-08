package response

import "time"

type VideoResponse struct {
	ID           string    `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	UserID       string    `json:"user_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Title        string    `json:"title" example:"Exploring the edge of the universe"`
	Description  string    `json:"description" example:"Join Luna Chen as she explores space."`
	Category     string    `json:"category" example:"Science"`
	Status       string    `json:"status" example:"complete"`
	Visibility   string    `json:"visibility" example:"public"`
	HLSUrl       string    `json:"hls_url" example:"https://s3.pipevid.com/hls/space/master.m3u8"`
	ThumbnailUrl string    `json:"thumbnail_url" example:"https://s3.pipevid.com/thumbnails/space.jpg"`
	Duration     string    `json:"duration" example:"18:42"`
	Views        int64     `json:"views" example:"24812"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type UploadUrlResponse struct {
	VideoID   string `json:"video_id"`
	UploadUrl string `json:"upload_url"`
	RawS3Key  string `json:"raw_s3_key"`
}
