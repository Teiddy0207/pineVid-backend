package mapper

import (
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
)

// ToVideoEntity converts CreateVideoUpload request DTO to Video Entity
func ToVideoEntity(userID string, req request.CreateVideoUpload, id, s3Key string) entity.Video {
	return entity.Video{
		ID:           id,
		UserID:       userID,
		Title:        req.Title,
		Description:  req.Description,
		Category:     req.Category,
		Status:       entity.VideoStatusPending,
		Visibility:   entity.VideoVisibilityPublic,
		RawS3Key:     s3Key,
		ThumbnailUrl: req.ThumbnailURL,
		Duration:     req.Duration,
	}
}

// ToVideoResponse converts Video Entity to VideoResponse DTO
func ToVideoResponse(v entity.Video) response.VideoResponse {
	return response.VideoResponse{
		ID:           v.ID,
		UserID:       v.UserID,
		Title:        v.Title,
		Description:  v.Description,
		Category:     v.Category,
		Status:       string(v.Status),
		Visibility:   string(v.Visibility),
		HLSUrl:       v.HLSUrl,
		ThumbnailUrl: v.ThumbnailUrl,
		Duration:     v.Duration,
		Views:        v.Views,
		CreatedAt:    v.CreatedAt,
		UpdatedAt:    v.UpdatedAt,
	}
}

// ToVideoResponses converts Video Entity slice to VideoResponse DTO slice
func ToVideoResponses(videos []entity.Video) []response.VideoResponse {
	res := make([]response.VideoResponse, len(videos))
	for i, v := range videos {
		res[i] = ToVideoResponse(v)
	}
	return res
}

// ToVideoPageResponse wraps Video Entity slice into standardized PageResponse DTO
func ToVideoPageResponse(videos []entity.Video, totalItems, page, limit int) response.PageResponse[response.VideoResponse] {
	totalPages := 0
	if limit > 0 {
		totalPages = (totalItems + limit - 1) / limit
	}

	return response.PageResponse[response.VideoResponse]{
		Success: true,
		Data:    ToVideoResponses(videos),
		Pagination: response.PaginationMeta{
			TotalItems:  totalItems,
			TotalPages:  totalPages,
			CurrentPage: page,
			Limit:       limit,
		},
	}
}
