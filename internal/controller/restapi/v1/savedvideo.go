package v1

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
)

// @Summary      Toggle save video (Watch Later)
// @Description  Add or remove a video from the caller's saved list
// @Tags         Video
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Video ID"
// @Success      200 {object} response.SaveVideoResponse
// @Failure      400 {object} response.Error
// @Failure      500 {object} response.Error
// @Router       /v1/videos/{id}/save [post]
func (r *V1) toggleSaveVideo(ctx *fiber.Ctx) error {
	videoID := ctx.Params("id")
	if videoID == "" {
		return errorResponse(ctx, http.StatusBadRequest, "video id required")
	}

	userID := getUserID(ctx)

	resDTO, err := r.sv.ToggleSaveVideo(ctx.UserContext(), userID, videoID)
	if err != nil {
		r.l.Error(err, "restapi - v1 - toggleSaveVideo")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to toggle saved video")
	}

	return ctx.Status(http.StatusOK).JSON(resDTO)
}

// @Summary      List saved videos
// @Description  Videos the caller has saved to Watch Later, most recently saved first
// @Tags         User
// @Produce      json
// @Security     BearerAuth
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Page limit" default(10)
// @Success      200 {object} response.PageResponse[response.VideoResponse]
// @Failure      500 {object} response.Error
// @Router       /v1/user/saved [get]
func (r *V1) listSavedVideos(ctx *fiber.Ctx) error {
	userID := getUserID(ctx)
	page := ctx.QueryInt("page", 1)
	limit := ctx.QueryInt("limit", 10)

	pageDTO, err := r.sv.ListSavedVideos(ctx.UserContext(), userID, page, limit)
	if err != nil {
		r.l.Error(err, "restapi - v1 - listSavedVideos")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to list saved videos")
	}

	return ctx.Status(http.StatusOK).JSON(pageDTO)
}
