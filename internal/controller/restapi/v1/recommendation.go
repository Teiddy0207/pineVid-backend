package v1

import (
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// @Summary      Get personalized recommendation video feed
// @Description  Get personalized video feed computed via Matrix Factorization + SGD
// @Tags         Recommendation
// @Accept       json
// @Produce      json
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Page limit" default(10)
// @Success      200 {object} response.PageResponse[response.RecommendedVideoItem]
// @Failure      500 {object} response.Error
// @Router       /v1/videos/feed/personalized [get]
func (r *V1) getPersonalizedFeed(ctx *fiber.Ctx) error {
	userID := getUserID(ctx)

	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(ctx.Query("limit", "10"))
	if limit < 1 || limit > 50 {
		limit = 10
	}

	pageResDTO, err := r.rc.GetPersonalizedFeed(ctx.UserContext(), userID, page, limit)
	if err != nil {
		r.l.Error(err, "restapi - v1 - getPersonalizedFeed")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to fetch personalized feed")
	}

	return ctx.Status(http.StatusOK).JSON(pageResDTO)
}
