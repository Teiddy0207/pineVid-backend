package request

type CreateCommentRequest struct {
	Content string `json:"content" validate:"required,min=1,max=1000"`
}
