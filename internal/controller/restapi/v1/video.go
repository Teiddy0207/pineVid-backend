package v1

import (
	"errors"
	"net/http"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	_ "github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/gofiber/fiber/v2"
)

// @Summary      List public videos
// @Description  Get a paginated list of public videos with optional category filter
// @Tags         Videos
// @Accept       json
// @Produce      json
// @Param        category query string false "Filter by category"
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Page limit" default(10)
// @Success      200 {object} response.VideoResponse
// @Failure      500 {object} response.Error
// @Router       /v1/videos [get]
func (r *V1) listPublicVideos(ctx *fiber.Ctx) error {
	category := ctx.Query("category")
	page := ctx.QueryInt("page", 1)
	limit := ctx.QueryInt("limit", 10)

	pageDTO, err := r.vd.ListPublicVideos(ctx.UserContext(), category, page, limit)
	if err != nil {
		r.l.Error(err, "restapi - v1 - listPublicVideos")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to list public videos")
	}

	return ctx.Status(http.StatusOK).JSON(pageDTO)
}

// @Summary      Get video by ID
// @Description  Get video details by ID
// @Tags         Videos
// @Accept       json
// @Produce      json
// @Param        id path string true "Video ID"
// @Success      200 {object} response.VideoResponse
// @Failure      404 {object} response.Error
// @Failure      500 {object} response.Error
// @Router       /v1/videos/{id} [get]
func (r *V1) getVideo(ctx *fiber.Ctx) error {
	id := ctx.Params("id")

	resDTO, err := r.vd.GetByID(ctx.UserContext(), id)
	if err != nil {
		r.l.Error(err, "restapi - v1 - getVideo")
		if errors.Is(err, entity.ErrVideoNotFound) {
			return errorResponse(ctx, http.StatusNotFound, "video not found")
		}
		return errorResponse(ctx, http.StatusInternalServerError, "internal server error")
	}

	return ctx.Status(http.StatusOK).JSON(resDTO)
}

// @Summary      Request Presigned S3 Upload URL
// @Description  Get a presigned S3 URL to upload raw video
// @Tags         Studio
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body request.CreateVideoUpload true "Video Upload Request"
// @Success      201 {object} response.UploadUrlResponse
// @Failure      400 {object} response.Error
// @Failure      500 {object} response.Error
// @Router       /v1/studio/upload-url [post]
func (r *V1) createVideoUpload(ctx *fiber.Ctx) error {
	var body request.CreateVideoUpload
	if err := ctx.BodyParser(&body); err != nil {
		r.l.Error(err, "restapi - v1 - createVideoUpload")
		return errorResponse(ctx, http.StatusBadRequest, "invalid request body")
	}

	userID := "mock-user-123" // In production, extract from JWT context

	uploadDTO, err := r.vd.CreateUpload(ctx.UserContext(), userID, body)
	if err != nil {
		r.l.Error(err, "restapi - v1 - createVideoUpload")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to create video upload")
	}

	return ctx.Status(http.StatusCreated).JSON(uploadDTO)
}

// @Summary      Confirm raw video upload
// @Description  Confirm that video file upload to S3 is complete and trigger transcode worker
// @Tags         Studio
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body request.ConfirmUpload true "Confirm Upload Request"
// @Success      200 {object} response.VideoResponse
// @Failure      400 {object} response.Error
// @Failure      500 {object} response.Error
// @Router       /v1/studio/confirm-upload [post]
func (r *V1) confirmVideoUpload(ctx *fiber.Ctx) error {
	var body request.ConfirmUpload
	if err := ctx.BodyParser(&body); err != nil {
		r.l.Error(err, "restapi - v1 - confirmVideoUpload")
		return errorResponse(ctx, http.StatusBadRequest, "invalid request body")
	}

	userID := "mock-user-123"

	resDTO, err := r.vd.ConfirmUpload(ctx.UserContext(), userID, body)
	if err != nil {
		r.l.Error(err, "restapi - v1 - confirmVideoUpload")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to confirm upload")
	}

	return ctx.Status(http.StatusOK).JSON(resDTO)
}

// @Summary      List studio videos
// @Description  Get videos uploaded by the authenticated streamer
// @Tags         Studio
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Page limit" default(10)
// @Success      200 {object} response.VideoResponse
// @Failure      500 {object} response.Error
// @Router       /v1/studio/videos [get]
func (r *V1) listStudioVideos(ctx *fiber.Ctx) error {
	userID := "mock-user-123"
	page := ctx.QueryInt("page", 1)
	limit := ctx.QueryInt("limit", 10)

	pageDTO, err := r.vd.ListStudioVideos(ctx.UserContext(), userID, page, limit)
	if err != nil {
		r.l.Error(err, "restapi - v1 - listStudioVideos")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to list studio videos")
	}

	return ctx.Status(http.StatusOK).JSON(pageDTO)
}

// @Summary      Publish studio video
// @Description  Set video visibility to public
// @Tags         Studio
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Video ID"
// @Success      200 {object} response.VideoResponse
// @Failure      500 {object} response.Error
// @Router       /v1/studio/videos/{id}/publish [post]
func (r *V1) publishVideo(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	userID := "mock-user-123"

	resDTO, err := r.vd.PublishVideo(ctx.UserContext(), userID, id)
	if err != nil {
		r.l.Error(err, "restapi - v1 - publishVideo")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to publish video")
	}

	return ctx.Status(http.StatusOK).JSON(resDTO)
}
