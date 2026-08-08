package v1

import (
	"errors"
	"net/http"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	_ "github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/gofiber/fiber/v2"
)

// @Summary      List active livestreams
// @Description  Get a paginated list of currently active livestreams
// @Tags         Livestream
// @Accept       json
// @Produce      json
// @Param        category query string false "Filter by category"
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Page limit" default(10)
// @Success      200 {object} response.LivestreamResponse
// @Failure      500 {object} response.Error
// @Router       /v1/live/streams [get]
func (r *V1) listActiveStreams(ctx *fiber.Ctx) error {
	category := ctx.Query("category")
	page := ctx.QueryInt("page", 1)
	limit := ctx.QueryInt("limit", 10)

	pageDTO, err := r.ls.ListActiveStreams(ctx.UserContext(), category, page, limit)
	if err != nil {
		r.l.Error(err, "restapi - v1 - listActiveStreams")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to list active streams")
	}

	return ctx.Status(http.StatusOK).JSON(pageDTO)
}

// @Summary      Get livestream by ID
// @Description  Get details of a livestream
// @Tags         Livestream
// @Accept       json
// @Produce      json
// @Param        id path string true "Livestream ID"
// @Success      200 {object} response.LivestreamResponse
// @Failure      404 {object} response.Error
// @Failure      500 {object} response.Error
// @Router       /v1/live/streams/{id} [get]
func (r *V1) getStream(ctx *fiber.Ctx) error {
	id := ctx.Params("id")

	resDTO, err := r.ls.GetStreamByID(ctx.UserContext(), id)
	if err != nil {
		r.l.Error(err, "restapi - v1 - getStream")
		if errors.Is(err, entity.ErrLivestreamNotFound) {
			return errorResponse(ctx, http.StatusNotFound, "livestream not found")
		}
		return errorResponse(ctx, http.StatusInternalServerError, "internal server error")
	}

	return ctx.Status(http.StatusOK).JSON(resDTO)
}

// @Summary      Get RTMP Stream Key
// @Description  Get stream key and RTMP server URL for OBS Studio
// @Tags         Studio
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} response.StreamKeyResponse
// @Failure      500 {object} response.Error
// @Router       /v1/studio/live/key [get]
func (r *V1) getStreamKey(ctx *fiber.Ctx) error {
	userID := "mock-user-123" // In production, extract from JWT context

	keyDTO, err := r.ls.GetStreamKey(ctx.UserContext(), userID)
	if err != nil {
		r.l.Error(err, "restapi - v1 - getStreamKey")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to get stream key")
	}

	return ctx.Status(http.StatusOK).JSON(keyDTO)
}

// @Summary      Reset RTMP Stream Key
// @Description  Generate a new stream key for OBS Studio
// @Tags         Studio
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} response.StreamKeyResponse
// @Failure      500 {object} response.Error
// @Router       /v1/studio/live/reset-key [post]
func (r *V1) resetStreamKey(ctx *fiber.Ctx) error {
	userID := "mock-user-123"

	keyDTO, err := r.ls.ResetStreamKey(ctx.UserContext(), userID)
	if err != nil {
		r.l.Error(err, "restapi - v1 - resetStreamKey")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to reset stream key")
	}

	return ctx.Status(http.StatusOK).JSON(keyDTO)
}

// @Summary      Authenticate RTMP Stream Key
// @Description  Webhook used by SRS Media Server (on_publish) to validate stream keys
// @Tags         Livestream
// @Accept       x-www-form-urlencoded
// @Produce      plain
// @Param        name formData string true "Stream key"
// @Success      200 {string} string "OK"
// @Failure      403 {string} string "invalid stream key"
// @Router       /v1/live/auth [post]
func (r *V1) authenticateRTMPStreamKey(ctx *fiber.Ctx) error {
	var body request.StreamKeyAuth
	if err := ctx.BodyParser(&body); err != nil {
		body.StreamKey = ctx.FormValue("name")
	}

	if body.StreamKey == "" {
		return ctx.Status(http.StatusForbidden).SendString("stream key required")
	}

	ok, err := r.ls.AuthenticateStreamKey(ctx.UserContext(), body)
	if err != nil || !ok {
		r.l.Error(err, "restapi - v1 - authenticateRTMPStreamKey")
		return ctx.Status(http.StatusForbidden).SendString("invalid stream key")
	}

	return ctx.Status(http.StatusOK).SendString("OK")
}
