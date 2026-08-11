package v1

import (
	"net/http"
	"strconv"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/gofiber/fiber/v2"
)

// @Summary      Create video comment
// @Description  Add a new comment under a video
// @Tags         Comment
// @Accept       json
// @Produce      json
// @Param        id path string true "Video ID"
// @Param        request body request.CreateCommentRequest true "Create Comment Request"
// @Success      201 {object} response.CommentResponse
// @Failure      400 {object} response.Error
// @Failure      500 {object} response.Error
// @Router       /v1/videos/{id}/comments [post]
func (r *V1) createComment(ctx *fiber.Ctx) error {
	videoID := ctx.Params("id")
	if videoID == "" {
		return errorResponse(ctx, http.StatusBadRequest, "video id required")
	}

	var body request.CreateCommentRequest
	if err := ctx.BodyParser(&body); err != nil {
		r.l.Error(err, "restapi - v1 - createComment")
		return errorResponse(ctx, http.StatusBadRequest, "invalid request body")
	}

	if err := r.v.Struct(body); err != nil {
		return errorResponse(ctx, http.StatusBadRequest, err.Error())
	}

	userID := getUserID(ctx)
	userName := ctx.Get("X-User-Name")
	if userName == "" {
		if len(userID) >= 4 {
			userName = "Viewer #" + userID[len(userID)-4:]
		} else {
			userName = "Viewer #" + userID
		}
	}
	userAvatar := ctx.Get("X-User-Avatar")

	resDTO, err := r.cm.CreateComment(ctx.UserContext(), videoID, userID, userName, userAvatar, body)
	if err != nil {
		r.l.Error(err, "restapi - v1 - createComment")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to post comment")
	}

	return ctx.Status(http.StatusCreated).JSON(resDTO)
}

// @Summary      List video comments
// @Description  Get paginated comments for a video
// @Tags         Comment
// @Accept       json
// @Produce      json
// @Param        id path string true "Video ID"
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Page limit" default(10)
// @Success      200 {object} response.PageResponse[response.CommentResponse]
// @Failure      500 {object} response.Error
// @Router       /v1/videos/{id}/comments [get]
func (r *V1) listVideoComments(ctx *fiber.Ctx) error {
	videoID := ctx.Params("id")
	if videoID == "" {
		return errorResponse(ctx, http.StatusBadRequest, "video id required")
	}

	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(ctx.Query("limit", "10"))
	if limit < 1 || limit > 50 {
		limit = 10
	}

	pageResDTO, err := r.cm.ListVideoComments(ctx.UserContext(), videoID, page, limit)
	if err != nil {
		r.l.Error(err, "restapi - v1 - listVideoComments")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to fetch comments")
	}

	return ctx.Status(http.StatusOK).JSON(pageResDTO)
}
