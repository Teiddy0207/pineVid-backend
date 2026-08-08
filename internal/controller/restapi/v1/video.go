package v1

import (
	"errors"
	"net/http"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/gofiber/fiber/v2"
)

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
