package v1

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/events"
	"github.com/gofiber/fiber/v2"
)

func getUserID(ctx *fiber.Ctx) string {
	if userID, ok := ctx.Locals("userID").(string); ok && userID != "" {
		return userID
	}
	return "mock-user-123"
}

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
	userID := ctx.Query("user_id")
	if userID == "" {
		userID = ctx.Query("creator_id")
	}
	page := ctx.QueryInt("page", 1)
	limit := ctx.QueryInt("limit", 10)

	pageDTO, err := r.vd.ListPublicVideos(ctx.UserContext(), userID, category, page, limit)
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

	userID := getUserID(ctx)

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

	userID := getUserID(ctx)

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
	userID := getUserID(ctx)
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
func (r *V1) transcodeCallback(ctx *fiber.Ctx) error {
	var body request.TranscodeCallback
	if err := ctx.BodyParser(&body); err != nil {
		return errorResponse(ctx, http.StatusBadRequest, "invalid payload")
	}

	if err := r.v.Struct(body); err != nil {
		return errorResponse(ctx, http.StatusBadRequest, err.Error())
	}

	if err := r.vd.HandleTranscodeCallback(ctx.UserContext(), body.VideoID, body.Status, body.HLSMasterURL); err != nil {
		r.l.Error(err, "restapi - v1 - transcodeCallback")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to update transcode status")
	}

	// Broadcast SSE event to all connected clients
	if r.hub != nil {
		r.hub.Broadcast(events.VideoEvent{
			VideoID: body.VideoID,
			Status:  body.Status,
			HLSUrl:  body.HLSMasterURL,
		})
	}

	return ctx.Status(http.StatusOK).JSON(fiber.Map{
		"message": "Transcode status updated successfully",
	})
}

// sseVideoEvents streams real-time video status updates to the frontend via Server-Sent Events
func (r *V1) sseVideoEvents(ctx *fiber.Ctx) error {
	ctx.Context().Response.Header.Set("Access-Control-Allow-Origin", "*")
	ctx.Context().Response.Header.Set("Access-Control-Allow-Headers", "*")
	ctx.Context().Response.Header.Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	ctx.Context().Response.Header.Set("Content-Type", "text/event-stream")
	ctx.Context().Response.Header.Set("Cache-Control", "no-cache")
	ctx.Context().Response.Header.Set("Connection", "keep-alive")

	ctx.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		ch, unsub := r.hub.Subscribe()
		defer unsub()

		// Immediate heartbeat to flush 200 OK headers to browser EventSource
		fmt.Fprintf(w, ": connected\n\n")
		if err := w.Flush(); err != nil {
			return
		}

		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case ev, ok := <-ch:
				if !ok {
					return
				}
				data, _ := json.Marshal(ev)
				fmt.Fprintf(w, "data: %s\n\n", data)
				if err := w.Flush(); err != nil {
					return
				}
			case <-ticker.C:
				fmt.Fprintf(w, ": ping\n\n")
				if err := w.Flush(); err != nil {
					return
				}
			}
		}
	})

	return nil
}

func (r *V1) publishVideo(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	userID := getUserID(ctx)

	resDTO, err := r.vd.PublishVideo(ctx.UserContext(), userID, id)
	if err != nil {
		r.l.Error(err, "restapi - v1 - publishVideo")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to publish video")
	}

	return ctx.Status(http.StatusOK).JSON(resDTO)
}

// @Summary      Record video view count
// @Description  Increment video view count after >10s playback with IP + Device deduplication (TTL 60s)
// @Tags         Video
// @Accept       json
// @Produce      json
// @Param        id path string true "Video ID"
// @Success      200 {object} response.RecordViewResponse
// @Failure      400 {object} response.Error
// @Failure      500 {object} response.Error
// @Router       /v1/videos/{id}/views [post]
func (r *V1) recordVideoView(ctx *fiber.Ctx) error {
	videoID := ctx.Params("id")
	if videoID == "" {
		return errorResponse(ctx, http.StatusBadRequest, "video id required")
	}

	clientIP := ctx.IP()
	if xff := ctx.Get("X-Forwarded-For"); xff != "" {
		clientIP = strings.Split(xff, ",")[0]
	}

	deviceID := ctx.Get("X-Device-ID")
	if deviceID == "" {
		deviceID = ctx.Get("User-Agent")
	}

	recorded, newViews, err := r.vd.RecordView(ctx.UserContext(), videoID, clientIP, deviceID)
	if err != nil {
		r.l.Error(err, "restapi - v1 - recordVideoView")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to record video view")
	}

	// Best-effort: only logged-in viewers (real userID from OptionalAuth) get a
	// watch-history entry; anonymous views never had one, so this is additive.
	if userID, ok := ctx.Locals("userID").(string); ok && userID != "" {
		if err := r.hs.RecordWatch(ctx.UserContext(), userID, videoID, 0); err != nil {
			r.l.Error(err, "restapi - v1 - recordVideoView - RecordWatch")
		}
	}

	return ctx.Status(http.StatusOK).JSON(response.RecordViewResponse{
		VideoID:    videoID,
		Recorded:   recorded,
		TotalViews: newViews,
	})
}
