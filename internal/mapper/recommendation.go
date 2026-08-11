package mapper

import (
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
)

func ToRecommendedVideoItem(r entity.RecommendedVideo) response.RecommendedVideoItem {
	return response.RecommendedVideoItem{
		Video:          ToVideoResponse(r.Video),
		PredictedScore: r.PredictedScore,
	}
}

func ToRecommendedVideoItems(recs []entity.RecommendedVideo) []response.RecommendedVideoItem {
	res := make([]response.RecommendedVideoItem, len(recs))
	for i, r := range recs {
		res[i] = ToRecommendedVideoItem(r)
	}
	return res
}

func ToPersonalizedFeedPageResponse(recs []entity.RecommendedVideo, totalItems, page, limit int) response.PageResponse[response.RecommendedVideoItem] {
	totalPages := 0
	if limit > 0 {
		totalPages = (totalItems + limit - 1) / limit
	}

	return response.PageResponse[response.RecommendedVideoItem]{
		Success: true,
		Data:    ToRecommendedVideoItems(recs),
		Pagination: response.PaginationMeta{
			TotalItems:  totalItems,
			TotalPages:  totalPages,
			CurrentPage: page,
			Limit:       limit,
		},
	}
}
