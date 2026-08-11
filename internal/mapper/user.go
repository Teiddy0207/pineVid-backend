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
		CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
