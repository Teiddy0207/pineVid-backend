package request

type CreatePostRequest struct {
	Content  string `json:"content" validate:"required,min=1,max=2000"`
	ImageURL string `json:"image_url" validate:"omitempty"`
}
