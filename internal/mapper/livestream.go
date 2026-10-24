package mapper

import (
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/events"
)

func ToLivestreamResponse(ls entity.Livestream) response.LivestreamResponse {
	return response.LivestreamResponse{
		ID: ls.ID,
		Streamer: response.LivestreamStreamer{
			ID:     ls.UserID,
			Name:   ls.UserName,
			Avatar: ls.UserAvatar,
		},
		Title:        ls.Title,
		Category:     ls.Category,
		ThumbnailURL: ls.ThumbnailURL,
		IsLive:       ls.IsLive,
		HLSUrl:       ls.HLSUrl,
		ViewersCount: ls.ViewersCount,
		StartedAt:    ls.StartedAt,
		EndedAt:      ls.EndedAt,
	}
}

func ToLivestreamResponses(streams []entity.Livestream) []response.LivestreamResponse {
	res := make([]response.LivestreamResponse, len(streams))
	for i, s := range streams {
		res[i] = ToLivestreamResponse(s)
	}
	return res
}

func ToLivestreamPageResponse(streams []entity.Livestream, totalItems, page, limit int) response.PageResponse[response.LivestreamResponse] {
	totalPages := 0
	if limit > 0 {
		totalPages = (totalItems + limit - 1) / limit
	}

	return response.PageResponse[response.LivestreamResponse]{
		Success: true,
		Data:    ToLivestreamResponses(streams),
		Pagination: response.PaginationMeta{
			TotalItems:  totalItems,
			TotalPages:  totalPages,
			CurrentPage: page,
			Limit:       limit,
		},
	}
}

func ToChatMessageResponse(msg events.ChatMessage) response.ChatMessageResponse {
	return response.ChatMessageResponse{
		StreamID: msg.StreamID,
		Type:     msg.Type,
		User: response.ChatUser{
			Name:   msg.Username,
			Avatar: msg.Avatar,
		},
		Text:      msg.Text,
		Value:     msg.Value,
		CreatedAt: msg.CreatedAt,
	}
}

func ToLiveSessionResponse(sess entity.LiveSession) response.LiveSessionResponse {
	res := response.LiveSessionResponse{
		ID:            sess.ID,
		LivestreamID:  sess.LivestreamID,
		Title:         sess.Title,
		ThumbnailURL:  sess.ThumbnailURL,
		Category:      sess.Category,
		StartedAt:     sess.StartedAt,
		EndedAt:       sess.EndedAt,
		ReplayVideoID: sess.ReplayVideoID,

		Hearts:         sess.Stats.Hearts,
		ChatMessages:   sess.Stats.ChatMessages,
		UniqueChatters: sess.Stats.UniqueChatters,
	}
	if sess.EndedAt != nil {
		dur := int64(sess.EndedAt.Sub(sess.StartedAt).Seconds())
		res.DurationSecs = &dur
	}
	return res
}

func ToLiveSessionPageResponse(sessions []entity.LiveSession, totalItems, page, limit int) response.PageResponse[response.LiveSessionResponse] {
	totalPages := 0
	if limit > 0 {
		totalPages = (totalItems + limit - 1) / limit
	}
	data := make([]response.LiveSessionResponse, len(sessions))
	for i, s := range sessions {
		data[i] = ToLiveSessionResponse(s)
	}
	return response.PageResponse[response.LiveSessionResponse]{
		Success: true,
		Data:    data,
		Pagination: response.PaginationMeta{
			TotalItems:  totalItems,
			TotalPages:  totalPages,
			CurrentPage: page,
			Limit:       limit,
		},
	}
}

func ToStoredChatMessageResponse(msg entity.LiveChatMessage) response.ChatMessageResponse {
	return response.ChatMessageResponse{
		ID:       msg.ID,
		SentAt:   &msg.CreatedAt,
		StreamID: msg.SessionID,
		Type:     "chat",
		User: response.ChatUser{
			Name:   msg.Username,
			Avatar: msg.Avatar,
		},
		Text:      msg.Text,
		CreatedAt: msg.CreatedAt.Format("15:04:05"),
	}
}
