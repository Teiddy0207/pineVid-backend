package v1

import (
	"net/http"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/gofiber/fiber/v2"
)

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
