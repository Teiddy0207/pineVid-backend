package mapper

import (
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
)

// ToUserResponse converts a User entity to the UserResponse DTO.
func ToUserResponse(u entity.User) response.UserResponse {
	return response.UserResponse{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		Avatar:    u.Avatar,
		IsBanned:  u.IsBanned,
		CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func ToUserResponses(users []entity.User) []response.UserResponse {
	res := make([]response.UserResponse, len(users))
	for i, u := range users {
		res[i] = ToUserResponse(u)
	}
	return res
}

func ToUserPageResponse(users []entity.User, totalItems, page, limit int) response.PageResponse[response.UserResponse] {
	totalPages := 0
	if limit > 0 {
		totalPages = (totalItems + limit - 1) / limit
	}
	return response.PageResponse[response.UserResponse]{
		Success: true,
		Data:    ToUserResponses(users),
		Pagination: response.PaginationMeta{
			TotalItems:  totalItems,
			TotalPages:  totalPages,
			CurrentPage: page,
			Limit:       limit,
		},
	}
}
