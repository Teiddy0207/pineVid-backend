package mapper

import (
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
)

func ToNotificationResponse(n entity.Notification) response.NotificationResponse {
	return response.NotificationResponse{
		ID:     n.ID,
		UserID: n.UserID,
		Sender: response.NotificationSender{
			ID:     n.SenderID,
			Name:   n.SenderName,
			Avatar: n.SenderAvatar,
		},
		Type:      string(n.Type),
		Title:     n.Title,
		Message:   n.Message,
		TargetURL: n.TargetURL,
		IsRead:    n.IsRead,
		CreatedAt: n.CreatedAt.Format(time.RFC3339),
	}
}

func ToNotificationListResponse(notifs []entity.Notification, total, unreadCount, page, limit int) response.NotificationListResponse {
	var list []response.NotificationResponse
	for _, n := range notifs {
		list = append(list, ToNotificationResponse(n))
	}
	if list == nil {
		list = []response.NotificationResponse{}
	}

	totalPages := 0
	if limit > 0 {
		totalPages = (total + limit - 1) / limit
	}

	return response.NotificationListResponse{
		PageResponse: response.PageResponse[response.NotificationResponse]{
			Success: true,
			Data:    list,
			Pagination: response.PaginationMeta{
				TotalItems:  total,
				TotalPages:  totalPages,
				CurrentPage: page,
				Limit:       limit,
			},
		},
		UnreadCount: unreadCount,
	}
}
