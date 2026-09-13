package v1

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/gofiber/fiber/v2"
)

// @Summary      Create a wall post
// @Description  Publish a text (+ optional image) status update to the caller's own wall
// @Tags         Post
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body request.CreatePostRequest true "Create Post Request"
// @Success      201 {object} response.PostResponse
// @Failure      400 {object} response.Error
// @Failure      500 {object} response.Error
// @Router       /v1/posts [post]
func (r *V1) createPost(ctx *fiber.Ctx) error {
	var body request.CreatePostRequest
	if err := ctx.BodyParser(&body); err != nil {
		r.l.Error(err, "restapi - v1 - createPost")
		return errorResponse(ctx, http.StatusBadRequest, "invalid request body")
	}

	if err := r.v.Struct(body); err != nil {
		return errorResponse(ctx, http.StatusBadRequest, err.Error())
	}

	userID := getUserID(ctx)

	// Resolve the author's real name/avatar server-side instead of trusting
	// client-supplied headers (unlike createComment's X-User-Name fallback) —
	// a post's author identity is exactly what we already store for this user.
	userName := "Viewer"
	userAvatar := ""
	if user, err := r.u.GetUser(ctx.UserContext(), userID); err == nil {
		userName = user.Username
		userAvatar = user.Avatar
	}

	resDTO, err := r.ps.CreatePost(ctx.UserContext(), userID, userName, userAvatar, body)
	if err != nil {
		r.l.Error(err, "restapi - v1 - createPost")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to create post")
	}

	return ctx.Status(http.StatusCreated).JSON(resDTO)
}

// @Summary      List a user's wall posts
// @Description  Get paginated posts published by a specific user, newest first
// @Tags         Post
// @Accept       json
// @Produce      json
// @Param        id path string true "User ID"
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Page limit" default(20)
// @Success      200 {object} response.PageResponse[response.PostResponse]
// @Failure      400 {object} response.Error
// @Failure      500 {object} response.Error
// @Router       /v1/channels/{id}/posts [get]
func (r *V1) listUserPosts(ctx *fiber.Ctx) error {
	userID := ctx.Params("id")
	if userID == "" {
		return errorResponse(ctx, http.StatusBadRequest, "user id required")
	}

	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(ctx.Query("limit", "20"))
	if limit < 1 || limit > 50 {
		limit = 20
	}

	viewerID := getUserID(ctx)
	pageResDTO, err := r.ps.ListPostsByUser(ctx.UserContext(), userID, viewerID, page, limit)
	if err != nil {
		r.l.Error(err, "restapi - v1 - listUserPosts")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to fetch posts")
	}

	return ctx.Status(http.StatusOK).JSON(pageResDTO)
}

// @Summary      Toggle like on a post
// @Description  Like or unlike a wall post as the authenticated user
// @Tags         Post
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Post ID"
// @Success      200 {object} response.PostLikeResponse
// @Failure      400 {object} response.Error
// @Failure      500 {object} response.Error
// @Router       /v1/posts/{id}/like [post]
func (r *V1) togglePostLike(ctx *fiber.Ctx) error {
	postID := ctx.Params("id")
	if postID == "" {
		return errorResponse(ctx, http.StatusBadRequest, "post id required")
	}

	userID := getUserID(ctx)
	resDTO, err := r.ps.TogglePostLike(ctx.UserContext(), postID, userID)
	if err != nil {
		r.l.Error(err, "restapi - v1 - togglePostLike")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to toggle post like")
	}

	return ctx.Status(http.StatusOK).JSON(resDTO)
}

// @Summary      Delete a wall post
// @Description  Delete one of the caller's own posts
// @Tags         Post
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Post ID"
// @Success      200 {object} map[string]bool
// @Failure      403 {object} response.Error
// @Failure      404 {object} response.Error
// @Failure      500 {object} response.Error
// @Router       /v1/posts/{id} [delete]
func (r *V1) deletePost(ctx *fiber.Ctx) error {
	postID := ctx.Params("id")
	if postID == "" {
		return errorResponse(ctx, http.StatusBadRequest, "post id required")
	}

	userID := getUserID(ctx)
	if err := r.ps.DeletePost(ctx.UserContext(), userID, postID); err != nil {
		if errors.Is(err, entity.ErrPostForbidden) {
			return errorResponse(ctx, http.StatusForbidden, err.Error())
		}
		if errors.Is(err, entity.ErrPostNotFound) {
			return errorResponse(ctx, http.StatusNotFound, err.Error())
		}
		r.l.Error(err, "restapi - v1 - deletePost")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to delete post")
	}

	return ctx.Status(http.StatusOK).JSON(fiber.Map{"success": true})
}
