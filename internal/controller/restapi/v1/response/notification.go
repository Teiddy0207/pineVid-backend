package response

// NotificationSender is the nested sender sub-object within NotificationResponse
// (mirrors the VideoCreator / CommentUser / LivestreamStreamer convention).
type NotificationSender struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

type NotificationResponse struct {
	ID        string             `json:"id"`
	UserID    string             `json:"user_id"`
	Sender    NotificationSender `json:"sender"`
	Type      string             `json:"type"`
	Title     string             `json:"title"`
	Message   string             `json:"message"`
	TargetURL string             `json:"target_url"`
	IsRead    bool               `json:"is_read"`
	CreatedAt string             `json:"created_at"`
}

type NotificationListResponse struct {
	PageResponse[NotificationResponse]
	UnreadCount int `json:"unread_count"`
}
