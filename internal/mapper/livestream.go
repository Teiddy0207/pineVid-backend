package mapper

import (
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
)

func ToLivestreamResponse(ls entity.Livestream) response.LivestreamResponse {
	return response.LivestreamResponse{
		ID:           ls.ID,
		UserID:       ls.UserID,
		Title:        ls.Title,
		Category:     ls.Category,
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
