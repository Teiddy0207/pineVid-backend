package v1

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
)

// @Summary      Toggle like on video
// @Description  Toggle like status for a video
// @Tags         Like
// @Accept       json
// @Produce      json
// @Param        id path string true "Video ID"
// @Success      200 {object} response.LikeResponse
// @Failure      400 {object} response.Error
// @Failure      500 {object} response.Error
// @Router       /v1/videos/{id}/like [post]
func (r *V1) toggleLikeVideo(ctx *fiber.Ctx) error {
	videoID := ctx.Params("id")
	if videoID == "" {
		return errorResponse(ctx, http.StatusBadRequest, "video id required")
	}

	userID := getUserID(ctx)

	resDTO, err := r.lk.ToggleLikeVideo(ctx.UserContext(), userID, videoID)
	if err != nil {
		r.l.Error(err, "restapi - v1 - toggleLikeVideo")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to toggle video like")
	}

	return ctx.Status(http.StatusOK).JSON(resDTO)
}

// @Summary      Send heart to livestream
// @Description  Increment live stream heart count in real time
// @Tags         Like
// @Accept       json
// @Produce      json
// @Param        id path string true "Stream ID"
// @Success      200 {object} response.HeartResponse
// @Failure      400 {object} response.Error
// @Failure      500 {object} response.Error
// @Router       /v1/live/{id}/heart [post]
func (r *V1) heartStream(ctx *fiber.Ctx) error {
	streamID := ctx.Params("id")
	if streamID == "" {
		return errorResponse(ctx, http.StatusBadRequest, "stream id required")
	}

	resDTO, err := r.lk.HeartStream(ctx.UserContext(), streamID)
	if err != nil {
		r.l.Error(err, "restapi - v1 - heartStream")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to send stream heart")
	}

	return ctx.Status(http.StatusOK).JSON(resDTO)
}
