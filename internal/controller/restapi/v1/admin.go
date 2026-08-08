package v1

import (
	"net/http"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/gofiber/fiber/v2"
)

// @Summary      Get Admin Dashboard Metrics
// @Description  Get system overview statistics (Total Videos, Active Streams, Viewers, Bandwidth)
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} response.SystemDashboardResponse
// @Failure      500 {object} response.Error
// @Router       /v1/admin/dashboard [get]
func (r *V1) getAdminDashboard(ctx *fiber.Ctx) error {
	dbDTO, err := r.ad.GetDashboard(ctx.UserContext())
	if err != nil {
		r.l.Error(err, "restapi - v1 - getAdminDashboard")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to get dashboard metrics")
	}

	return ctx.Status(http.StatusOK).JSON(response.Response[response.SystemDashboardResponse]{
		Success: true,
		Data:    dbDTO,
	})
}

// @Summary      Get Transcode Workers Status
// @Description  Get real-time CPU, RAM, and current processing jobs of transcode worker nodes
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} response.WorkerStatusResponse
// @Failure      500 {object} response.Error
// @Router       /v1/admin/workers [get]
func (r *V1) getAdminWorkers(ctx *fiber.Ctx) error {
	workers, err := r.ad.GetWorkersStatus(ctx.UserContext())
	if err != nil {
		r.l.Error(err, "restapi - v1 - getAdminWorkers")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to get workers status")
	}

	return ctx.Status(http.StatusOK).JSON(response.Response[[]response.WorkerStatusResponse]{
		Success: true,
		Data:    workers,
	})
}

// @Summary      Ban active livestream
// @Description  Instantly terminate and ban an active livestream
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Livestream ID"
// @Success      200 {object} map[string]interface{}
// @Failure      500 {object} response.Error
// @Router       /v1/admin/streams/{id}/ban [post]
func (r *V1) banStream(ctx *fiber.Ctx) error {
	streamID := ctx.Params("id")

	if err := r.ad.BanStream(ctx.UserContext(), streamID); err != nil {
		r.l.Error(err, "restapi - v1 - banStream")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to ban livestream")
	}

	return ctx.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "livestream banned successfully",
	})
}

// @Summary      Ban video
// @Description  Remove or set video visibility to private due to terms violation
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Video ID"
// @Success      200 {object} map[string]interface{}
// @Failure      500 {object} response.Error
// @Router       /v1/admin/videos/{id}/ban [post]
func (r *V1) banVideo(ctx *fiber.Ctx) error {
	videoID := ctx.Params("id")

	if err := r.ad.BanVideo(ctx.UserContext(), videoID); err != nil {
		r.l.Error(err, "restapi - v1 - banVideo")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to ban video")
	}

	return ctx.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "video banned successfully",
	})
}
