package request

type StreamKeyAuth struct {
	StreamKey string `json:"name" form:"name" validate:"required"` // SRS sends stream key as 'name' form parameter
}

type UpdateLivestreamInfo struct {
	Title    string `json:"title" validate:"omitempty,min=3,max=255"`
	Category string `json:"category"`
}

type SendChatMessage struct {
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
	Text     string `json:"text" validate:"required"`
}
