package v1

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
)

const maxHistoryLimit = 50

// @Summary      Get current user's watch history
// @Description  Paginated list of videos the current user has watched, most recent first
// @Tags         History
// @Produce      json
// @Security     BearerAuth
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Page limit" default(10)
// @Success      200 {object} response.PageResponse[response.VideoResponse]
// @Failure      401 {object} response.Error
// @Failure      500 {object} response.Error
// @Router       /v1/user/history [get]
func (r *V1) getWatchHistory(ctx *fiber.Ctx) error {
	userID, ok := ctx.Locals("userID").(string)
	if !ok || userID == "" {
		return errorResponse(ctx, http.StatusUnauthorized, "unauthorized")
	}

	page := ctx.QueryInt("page", 1)
	limit := ctx.QueryInt("limit", 10)
	if limit > maxHistoryLimit {
		limit = maxHistoryLimit
	}

	resDTO, err := r.hs.ListHistory(ctx.UserContext(), userID, page, limit)
	if err != nil {
		r.l.Error(err, "restapi - v1 - getWatchHistory")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to load watch history")
	}

	return ctx.Status(http.StatusOK).JSON(resDTO)
}
