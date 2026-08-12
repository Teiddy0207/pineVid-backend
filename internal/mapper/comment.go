package mapper

import (
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
)

func ToCommentEntity(videoID, userID, userName, userAvatar string, req request.CreateCommentRequest, id string) entity.Comment {
	return entity.Comment{
		ID:         id,
		VideoID:    videoID,
		UserID:     userID,
		UserName:   userName,
		UserAvatar: userAvatar,
		Content:    req.Content,
	}
}

func ToCommentResponse(c entity.Comment) response.CommentResponse {
	return response.CommentResponse{
		ID:      c.ID,
		VideoID: c.VideoID,
		User: response.CommentUser{
			ID:     c.UserID,
			Name:   c.UserName,
			Avatar: c.UserAvatar,
		},
		Content:   c.Content,
		CreatedAt: c.CreatedAt,
	}
}

func ToCommentResponses(comments []entity.Comment) []response.CommentResponse {
	res := make([]response.CommentResponse, len(comments))
	for i, c := range comments {
		res[i] = ToCommentResponse(c)
	}
	return res
}

func ToCommentPageResponse(comments []entity.Comment, totalItems, page, limit int) response.PageResponse[response.CommentResponse] {
	totalPages := 0
	if limit > 0 {
		totalPages = (totalItems + limit - 1) / limit
	}

	return response.PageResponse[response.CommentResponse]{
		Success: true,
		Data:    ToCommentResponses(comments),
		Pagination: response.PaginationMeta{
			TotalItems:  totalItems,
			TotalPages:  totalPages,
			CurrentPage: page,
			Limit:       limit,
		},
	}
}
