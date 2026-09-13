package mapper

import (
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
)

func ToPostEntity(userID, userName, userAvatar string, req request.CreatePostRequest, id string) entity.Post {
	return entity.Post{
		ID:         id,
		UserID:     userID,
		UserName:   userName,
		UserAvatar: userAvatar,
		Content:    req.Content,
		ImageURL:   req.ImageURL,
	}
}

func ToPostResponse(p entity.Post) response.PostResponse {
	return response.PostResponse{
		ID: p.ID,
		Author: response.PostAuthor{
			ID:     p.UserID,
			Name:   p.UserName,
			Avatar: p.UserAvatar,
		},
		Content:   p.Content,
		ImageURL:  p.ImageURL,
		CreatedAt: p.CreatedAt,
	}
}

func ToPostResponses(posts []entity.Post) []response.PostResponse {
	res := make([]response.PostResponse, len(posts))
	for i, p := range posts {
		res[i] = ToPostResponse(p)
	}
	return res
}

func ToPostPageResponse(posts []entity.Post, totalItems, page, limit int) response.PageResponse[response.PostResponse] {
	totalPages := 0
	if limit > 0 {
		totalPages = (totalItems + limit - 1) / limit
	}

	return response.PageResponse[response.PostResponse]{
		Success: true,
		Data:    ToPostResponses(posts),
		Pagination: response.PaginationMeta{
			TotalItems:  totalItems,
			TotalPages:  totalPages,
			CurrentPage: page,
			Limit:       limit,
		},
	}
}
