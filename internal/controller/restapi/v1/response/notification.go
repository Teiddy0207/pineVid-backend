package response

type NotificationResponse struct {
	ID           string `json:"id"`
	UserID       string `json:"user_id"`
	SenderID     string `json:"sender_id"`
	SenderName   string `json:"sender_name"`
	SenderAvatar string `json:"sender_avatar"`
	Type         string `json:"type"`
	Title        string `json:"title"`
	Message      string `json:"message"`
	TargetURL    string `json:"target_url"`
	IsRead       bool   `json:"is_read"`
	CreatedAt    string `json:"created_at"`
}

type NotificationListResponse struct {
	PageResponse[NotificationResponse]
	UnreadCount int `json:"unread_count"`
}
