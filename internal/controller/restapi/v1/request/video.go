package request

type CreateVideoUpload struct {
	Title        string `json:"title" validate:"required,min=3,max=255"`
	Description  string `json:"description" validate:"max=1000"`
	Category     string `json:"category"`
	FileName     string `json:"file_name" validate:"required"`
	ThumbnailURL string `json:"thumbnail_url"`
}

type UpdateVideo struct {
	Title        string `json:"title" validate:"omitempty,min=3,max=255"`
	Description  string `json:"description" validate:"omitempty,max=1000"`
	Category     string `json:"category"`
	Visibility   string `json:"visibility" validate:"omitempty,oneof=public private unlisted"`
	ThumbnailURL string `json:"thumbnail_url"`
}

type ConfirmUpload struct {
	VideoID string `json:"video_id" validate:"required"`
}

type TranscodeCallback struct {
	VideoID      string `json:"video_id" validate:"required"`
	Status       string `json:"status" validate:"required,oneof=complete failed"`
	HLSMasterURL string `json:"hls_master_url"`
	ErrorMessage string `json:"error_message,omitempty"`
}

