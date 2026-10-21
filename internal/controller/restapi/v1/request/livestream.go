package request

type StreamKeyAuth struct {
	// SRS's on_publish/on_unpublish HTTP callback POSTs a JSON body with the
	// stream key under "stream" (e.g. {"action":"on_publish",...,"stream":"sk_live_..."}).
	StreamKey string `json:"stream" form:"name" validate:"required"`
}

type UpdateLivestreamInfo struct {
	Title        string `json:"title" validate:"omitempty,min=3,max=255"`
	Category     string `json:"category"`
	ThumbnailURL string `json:"thumbnail_url" validate:"omitempty,url,max=512"`
}

// SendChatMessage — the sender's name/avatar are taken from the
// authenticated account, not from the request.
type SendChatMessage struct {
	Text string `json:"text" validate:"required"`
}
