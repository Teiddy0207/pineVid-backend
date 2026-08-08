package request

type CreateVideoUpload struct {
	Title       string `json:"title" validate:"required,min=3,max=255"`
	Description string `json:"description" validate:"max=1000"`
	Category    string `json:"category"`
	FileName    string `json:"file_name" validate:"required"`
}

type UpdateVideo struct {
	Title       string `json:"title" validate:"omitempty,min=3,max=255"`
	Description string `json:"description" validate:"omitempty,max=1000"`
	Category    string `json:"category"`
	Visibility  string `json:"visibility" validate:"omitempty,oneof=public private unlisted"`
}

type ConfirmUpload struct {
	VideoID string `json:"video_id" validate:"required"`
}
