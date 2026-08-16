package v1

import (
	"net/http"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
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

// @Summary      Transcode worker heartbeat
// @Description  Internal webhook: transcode worker processes self-report their live status here
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Param        request body request.WorkerHeartbeat true "Worker heartbeat payload"
// @Success      200 {object} map[string]interface{}
// @Failure      400 {object} response.Error
// @Router       /v1/admin/workers/heartbeat [post]
func (r *V1) recordWorkerHeartbeat(ctx *fiber.Ctx) error {
	var body request.WorkerHeartbeat
	if err := ctx.BodyParser(&body); err != nil {
		return errorResponse(ctx, http.StatusBadRequest, "invalid request body")
	}
	if err := r.v.Struct(body); err != nil {
		return errorResponse(ctx, http.StatusBadRequest, err.Error())
	}

	hb := entity.WorkerHeartbeat{
		WorkerID:   body.WorkerID,
		Status:     body.Status,
		CurrentJob: body.CurrentJob,
		CPUUsage:   body.CPUUsage,
		RAMUsageMB: body.RAMUsageMB,
	}

	if err := r.ad.RecordHeartbeat(ctx.UserContext(), hb); err != nil {
		r.l.Error(err, "restapi - v1 - recordWorkerHeartbeat")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to record heartbeat")
	}

	return ctx.Status(http.StatusOK).JSON(fiber.Map{"success": true})
}

// @Summary      List users
// @Description  Paginated list of all registered users, for moderation
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Page limit" default(10)
// @Success      200 {object} response.PageResponse[response.UserResponse]
// @Failure      500 {object} response.Error
// @Router       /v1/admin/users [get]
func (r *V1) listAdminUsers(ctx *fiber.Ctx) error {
	page := ctx.QueryInt("page", 1)
	limit := ctx.QueryInt("limit", 10)
	if limit > 50 {
		limit = 50
	}

	pageDTO, err := r.ad.ListUsers(ctx.UserContext(), page, limit)
	if err != nil {
		r.l.Error(err, "restapi - v1 - listAdminUsers")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to list users")
	}

	return ctx.Status(http.StatusOK).JSON(pageDTO)
}

// @Summary      Ban user account
// @Description  Lock a user account, preventing further login
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "User ID"
// @Success      200 {object} map[string]interface{}
// @Failure      500 {object} response.Error
// @Router       /v1/admin/users/{id}/ban [post]
func (r *V1) banUser(ctx *fiber.Ctx) error {
	userID := ctx.Params("id")

	if err := r.ad.BanUser(ctx.UserContext(), userID); err != nil {
		r.l.Error(err, "restapi - v1 - banUser")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to ban user")
	}

	return ctx.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "user banned successfully",
	})
}

// @Summary      Unban user account
// @Description  Unlock a previously banned user account
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "User ID"
// @Success      200 {object} map[string]interface{}
// @Failure      500 {object} response.Error
// @Router       /v1/admin/users/{id}/unban [post]
func (r *V1) unbanUser(ctx *fiber.Ctx) error {
	userID := ctx.Params("id")

	if err := r.ad.UnbanUser(ctx.UserContext(), userID); err != nil {
		r.l.Error(err, "restapi - v1 - unbanUser")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to unban user")
	}

	return ctx.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "user unbanned successfully",
	})
}
