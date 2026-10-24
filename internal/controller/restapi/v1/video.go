package v1

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/events"
	"github.com/gofiber/fiber/v2"
)

// getUserID returns the authenticated caller's userID. All callers of this
// helper sit behind middleware.Auth, which already rejects the request
// before the handler runs if no valid token was presented — so a missing
// value here would indicate a routing bug, not a legitimate anonymous caller.
func getUserID(ctx *fiber.Ctx) string {
	userID, _ := ctx.Locals("userID").(string)
	return userID
}

// videoViewable guards per-video endpoints: it writes a 404 (or 500) and
// returns false when the caller may not access videoID — private, banned or
// unfinished videos of someone else (see entity.Video.ViewableBy).
// Usage: if ok, err := r.videoViewable(ctx, id); !ok { return err }
func (r *V1) videoViewable(ctx *fiber.Ctx, videoID string) (bool, error) {
	userID, _ := ctx.Locals("userID").(string)

	err := r.vd.EnsureViewable(ctx.UserContext(), videoID, userID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, entity.ErrVideoNotFound) {
		return false, errorResponse(ctx, http.StatusNotFound, "video not found")
	}

	r.l.Error(err, "restapi - v1 - videoViewable")
	return false, errorResponse(ctx, http.StatusInternalServerError, "internal server error")
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
	query := ctx.Query("q")
	if query == "" {
		query = ctx.Query("query")
	}
	page := ctx.QueryInt("page", 1)
	limit := ctx.QueryInt("limit", 10)

	pageDTO, err := r.vd.ListPublicVideos(ctx.UserContext(), userID, category, query, page, limit)
	if err != nil {
		r.l.Error(err, "restapi - v1 - listPublicVideos")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to list public videos")
	}

	return ctx.Status(http.StatusOK).JSON(pageDTO)
}

// @Summary      List trending videos
// @Description  Get public videos ranked by view count over a recent window ("24h" or "7d"), instead of lifetime total views
// @Tags         Videos
// @Accept       json
// @Produce      json
// @Param        window query string false "Ranking window: 24h or 7d" default(24h)
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Page limit" default(10)
// @Success      200 {object} response.VideoResponse
// @Failure      500 {object} response.Error
// @Router       /v1/videos/trending [get]
func (r *V1) listTrendingVideos(ctx *fiber.Ctx) error {
	window := ctx.Query("window", "24h")
	if window != "7d" {
		window = "24h"
	}
	userID := getUserID(ctx)
	page := ctx.QueryInt("page", 1)
	limit := ctx.QueryInt("limit", 10)

	pageDTO, err := r.vd.GetTrending(ctx.UserContext(), window, userID, page, limit)
	if err != nil {
		r.l.Error(err, "restapi - v1 - listTrendingVideos")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to list trending videos")
	}

	return ctx.Status(http.StatusOK).JSON(pageDTO)
}

func (r *V1) listReels(ctx *fiber.Ctx) error {
	window := ctx.Query("window", "7d")
	if window != "24h" {
		window = "7d"
	}
	userID := getUserID(ctx)
	page := ctx.QueryInt("page", 1)
	limit := ctx.QueryInt("limit", 10)

	pageDTO, err := r.vd.GetReelsFeed(ctx.UserContext(), userID, window, page, limit)
	if err != nil {
		r.l.Error(err, "restapi - v1 - listReels")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to list reels")
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
	userID, _ := ctx.Locals("userID").(string)

	resDTO, err := r.vd.GetByID(ctx.UserContext(), id, userID)
	if err != nil {
		if errors.Is(err, entity.ErrVideoNotFound) {
			return errorResponse(ctx, http.StatusNotFound, "video not found")
		}
		r.l.Error(err, "restapi - v1 - getVideo")
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

	if err := r.v.Struct(body); err != nil {
		return errorResponse(ctx, http.StatusBadRequest, err.Error())
	}

	userID := getUserID(ctx)

	uploadDTO, err := r.vd.CreateUpload(ctx.UserContext(), userID, body)
	if err != nil {
		r.l.Error(err, "restapi - v1 - createVideoUpload")
		if errors.Is(err, entity.ErrUnsupportedVideoFormat) {
			return errorResponse(ctx, http.StatusBadRequest, err.Error())
		}
		return errorResponse(ctx, http.StatusInternalServerError, "failed to create video upload")
	}

	return ctx.Status(http.StatusCreated).JSON(uploadDTO)
}

// @Summary      Request upload URL for an image
// @Description  Get a presigned PUT URL for an avatar/thumbnail/post image plus the public URL it will be served from
// @Tags         Uploads
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body request.CreateImageUpload true "Image Upload Request"
// @Success      201 {object} response.ImageUploadResponse
// @Failure      400 {object} response.Error
// @Failure      500 {object} response.Error
// @Router       /v1/uploads/image-url [post]
func (r *V1) createImageUpload(ctx *fiber.Ctx) error {
	var body request.CreateImageUpload
	if err := ctx.BodyParser(&body); err != nil {
		r.l.Error(err, "restapi - v1 - createImageUpload")
		return errorResponse(ctx, http.StatusBadRequest, "invalid request body")
	}

	if err := r.v.Struct(body); err != nil {
		return errorResponse(ctx, http.StatusBadRequest, err.Error())
	}

	uploadDTO, err := r.vd.CreateImageUpload(ctx.UserContext(), getUserID(ctx), body)
	if err != nil {
		r.l.Error(err, "restapi - v1 - createImageUpload")
		if errors.Is(err, entity.ErrUnsupportedImageFormat) {
			return errorResponse(ctx, http.StatusBadRequest, err.Error())
		}
		return errorResponse(ctx, http.StatusInternalServerError, "failed to create image upload")
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
		if errors.Is(err, entity.ErrVideoForbidden) {
			return errorResponse(ctx, http.StatusForbidden, err.Error())
		}
		if errors.Is(err, entity.ErrVideoNotFound) {
			return errorResponse(ctx, http.StatusNotFound, err.Error())
		}
		if errors.Is(err, entity.ErrTranscodeQueueUnavailable) {
			return errorResponse(ctx, http.StatusServiceUnavailable, entity.ErrTranscodeQueueUnavailable.Error())
		}
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
	query := ctx.Query("q")

	pageDTO, err := r.vd.ListStudioVideos(ctx.UserContext(), userID, query, page, limit)
	if err != nil {
		r.l.Error(err, "restapi - v1 - listStudioVideos")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to list studio videos")
	}

	return ctx.Status(http.StatusOK).JSON(pageDTO)
}

// transcodeWebhookResponse is the reply to a transcode webhook: applied=false
// with a reason is a successful no-op (docs/message-contract.md §4.4).
type transcodeWebhookResponse struct {
	Applied bool   `json:"applied"`
	Reason  string `json:"reason,omitempty"`
}

// transcodeWebhook receives signed status events from the transcode worker
// (POST /internal/webhooks/transcode). The signature is checked by
// middleware.RequireWebhookSignature before this runs.
func (r *V1) transcodeWebhook(ctx *fiber.Ctx) error {
	key := ctx.Get("Idempotency-Key")
	if key == "" {
		return errorResponse(ctx, http.StatusBadRequest, "Idempotency-Key header required")
	}

	var body request.TranscodeWebhook
	if err := ctx.BodyParser(&body); err != nil {
		return errorResponse(ctx, http.StatusBadRequest, "invalid payload")
	}
	if err := r.v.Struct(body); err != nil {
		return errorResponse(ctx, http.StatusBadRequest, err.Error())
	}

	ev := entity.TranscodeEvent{
		IdempotencyKey: key,
		JobID:          body.JobID,
		VideoID:        body.VideoID,
		Status:         entity.TranscodeEventStatus(body.Status),
		HLSMasterURL:   body.HLSMasterURL,
		ErrorMessage:   body.ErrorMessage,
	}

	res, err := r.vd.HandleTranscodeEvent(ctx.UserContext(), ev)
	if err != nil {
		if errors.Is(err, entity.ErrVideoNotFound) {
			return errorResponse(ctx, http.StatusNotFound, err.Error())
		}
		if errors.Is(err, entity.ErrInvalidIdempotencyKey) {
			return errorResponse(ctx, http.StatusBadRequest, err.Error())
		}
		r.l.Error(err, "restapi - v1 - transcodeWebhook")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to update transcode status")
	}

	if res.Applied {
		r.broadcastTranscodeStatus(res.Before.UserID, ev)
	}

	return ctx.Status(http.StatusOK).JSON(transcodeWebhookResponse{Applied: res.Applied, Reason: res.Reason})
}

// broadcastTranscodeStatus pushes an applied transcode event to the owner's
// open Studio tabs (SSE), as the video status the frontend knows.
func (r *V1) broadcastTranscodeStatus(ownerID string, ev entity.TranscodeEvent) {
	if r.hub == nil {
		return
	}

	status := entity.VideoStatusProcessing
	switch ev.Status {
	case entity.TranscodeEventCompleted:
		status = entity.VideoStatusComplete
	case entity.TranscodeEventFailed:
		status = entity.VideoStatusFailed
	}

	r.hub.Broadcast(events.VideoEvent{
		UserID:  ownerID,
		VideoID: ev.VideoID,
		Status:  string(status),
		HLSUrl:  ev.HLSMasterURL,
	})
}

// sseVideoEvents streams real-time video status updates to the frontend via Server-Sent Events
// Only events for the caller's own videos are delivered (SSEAuth sets userID).
func (r *V1) sseVideoEvents(ctx *fiber.Ctx) error {
	userID := getUserID(ctx)

	ctx.Context().Response.Header.Set("Content-Type", "text/event-stream")
	ctx.Context().Response.Header.Set("Cache-Control", "no-cache")
	ctx.Context().Response.Header.Set("Connection", "keep-alive")

	ctx.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		ch, unsub := r.hub.Subscribe(userID)
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
	userID := getUserID(ctx)

	resDTO, err := r.vd.PublishVideo(ctx.UserContext(), userID, id)
	if err != nil {
		r.l.Error(err, "restapi - v1 - publishVideo")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to publish video")
	}

	return ctx.Status(http.StatusOK).JSON(resDTO)
}

// @Summary      Start a view session
// @Description  Called when playback starts. Returns a one-time ticket to redeem at POST /videos/{id}/views after at least min_watch_seconds of watching; the wait is checked on the server clock.
// @Tags         Video
// @Produce      json
// @Param        id path string true "Video ID"
// @Success      200 {object} response.ViewSessionResponse
// @Failure      404 {object} response.Error
// @Router       /v1/videos/{id}/view-session [post]
func (r *V1) startViewSession(ctx *fiber.Ctx) error {
	videoID := ctx.Params("id")
	userID, _ := ctx.Locals("userID").(string)

	ticket, err := r.vd.StartViewSession(ctx.UserContext(), videoID, userID)
	if err != nil {
		if errors.Is(err, entity.ErrVideoNotFound) {
			return errorResponse(ctx, http.StatusNotFound, "video not found")
		}
		r.l.Error(err, "restapi - v1 - startViewSession")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to start view session")
	}

	return ctx.Status(http.StatusOK).JSON(response.ViewSessionResponse{
		Ticket:          ticket,
		MinWatchSeconds: int(entity.MinWatchForView.Seconds()),
	})
}

// @Summary      Record video view count
// @Description  Redeem a view-session ticket once the viewer has watched long enough (>=10s, checked server-side); then counts 1 view with IP + device deduplication (60s) and a per-IP hourly cap
// @Tags         Video
// @Accept       json
// @Produce      json
// @Param        id path string true "Video ID"
// @Param        request body request.RecordView true "View ticket"
// @Success      200 {object} response.RecordViewResponse
// @Failure      400 {object} response.Error
// @Failure      425 {object} response.Error "minimum watch time not reached yet"
// @Router       /v1/videos/{id}/views [post]
func (r *V1) recordVideoView(ctx *fiber.Ctx) error {
	videoID := ctx.Params("id")
	if videoID == "" {
		return errorResponse(ctx, http.StatusBadRequest, "video id required")
	}

	// The ticket may be missing: RecordView checks the video is viewable
	// first (404 for a private one) and only then rejects the ticket.
	var body request.RecordView
	_ = ctx.BodyParser(&body)

	// ctx.IP() only honours X-Forwarded-For from HTTP_TRUSTED_PROXIES (see
	// httpserver config); reading the header directly would let any client
	// pick a fresh fake IP per request and bypass view dedup.
	clientIP := ctx.IP()

	recorded, newViews, err := r.vd.RecordView(ctx.UserContext(), videoID, body.Ticket, getUserID(ctx), clientIP, ctx.Get("User-Agent"))
	if err != nil {
		if errors.Is(err, entity.ErrVideoNotFound) {
			return errorResponse(ctx, http.StatusNotFound, "video not found")
		}
		if errors.Is(err, entity.ErrViewTicketTooEarly) {
			return errorResponse(ctx, http.StatusTooEarly, err.Error())
		}
		if errors.Is(err, entity.ErrViewTicketInvalid) {
			return errorResponse(ctx, http.StatusBadRequest, "view ticket required (POST /videos/{id}/view-session first): "+err.Error())
		}
		r.l.Error(err, "restapi - v1 - recordVideoView")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to record video view")
	}

	// A validated view ticket also records history, even when view dedup
	// prevents another increment of the public counter.
	if userID := getUserID(ctx); userID != "" && r.hs != nil {
		if err := r.hs.RecordWatch(ctx.UserContext(), userID, videoID, int(entity.MinWatchForView.Seconds())); err != nil {
			r.l.Error(err, "restapi - v1 - recordVideoView - history")
		}
	}

	return ctx.Status(http.StatusOK).JSON(response.RecordViewResponse{
		VideoID:    videoID,
		Recorded:   recorded,
		TotalViews: newViews,
	})
}

// @Summary      Update studio video details
// @Description  Update title, description, category, and visibility of a studio video
// @Tags         Studio
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Video ID"
// @Param        request body request.UpdateVideo true "Video update details"
// @Success      200 {object} response.VideoResponse
// @Failure      400 {object} response.Error
// @Failure      500 {object} response.Error
// @Router       /v1/studio/videos/{id} [put]
func (r *V1) updateVideo(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	userID := getUserID(ctx)

	var body request.UpdateVideo
	if err := ctx.BodyParser(&body); err != nil {
		r.l.Error(err, "restapi - v1 - updateVideo")
		return errorResponse(ctx, http.StatusBadRequest, "invalid request body")
	}

	if err := r.v.Struct(body); err != nil {
		return errorResponse(ctx, http.StatusBadRequest, err.Error())
	}

	resDTO, err := r.vd.UpdateVideo(ctx.UserContext(), userID, id, body)
	if err != nil {
		r.l.Error(err, "restapi - v1 - updateVideo")
		if errors.Is(err, entity.ErrVideoForbidden) {
			return errorResponse(ctx, http.StatusForbidden, err.Error())
		}
		if errors.Is(err, entity.ErrVideoNotFound) {
			return errorResponse(ctx, http.StatusNotFound, err.Error())
		}
		return errorResponse(ctx, http.StatusInternalServerError, "failed to update video")
	}

	return ctx.Status(http.StatusOK).JSON(resDTO)
}

// @Summary      Update video thumbnail
// @Description  Update thumbnail URL of a studio video
// @Tags         Studio
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Video ID"
// @Param        request body request.UpdateThumbnail true "Thumbnail update payload"
// @Success      200 {object} response.VideoResponse
// @Failure      400 {object} response.Error
// @Failure      500 {object} response.Error
// @Router       /v1/studio/videos/{id}/thumbnail [put]
func (r *V1) updateThumbnail(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	userID := getUserID(ctx)

	var body request.UpdateThumbnail
	if err := ctx.BodyParser(&body); err != nil {
		r.l.Error(err, "restapi - v1 - updateThumbnail")
		return errorResponse(ctx, http.StatusBadRequest, "invalid request body")
	}

	resDTO, err := r.vd.UpdateThumbnail(ctx.UserContext(), userID, id, body)
	if err != nil {
		r.l.Error(err, "restapi - v1 - updateThumbnail")
		if errors.Is(err, entity.ErrVideoForbidden) {
			return errorResponse(ctx, http.StatusForbidden, err.Error())
		}
		if errors.Is(err, entity.ErrVideoNotFound) {
			return errorResponse(ctx, http.StatusNotFound, err.Error())
		}
		return errorResponse(ctx, http.StatusInternalServerError, "failed to update thumbnail")
	}

	return ctx.Status(http.StatusOK).JSON(resDTO)
}

// @Summary      Delete studio video
// @Description  Delete video by ID from studio
// @Tags         Studio
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Video ID"
// @Success      200 {object} map[string]string
// @Failure      500 {object} response.Error
// @Router       /v1/studio/videos/{id} [delete]
func (r *V1) deleteVideo(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	userID := getUserID(ctx)

	if err := r.vd.DeleteVideo(ctx.UserContext(), userID, id); err != nil {
		r.l.Error(err, "restapi - v1 - deleteVideo")
		if errors.Is(err, entity.ErrVideoForbidden) {
			return errorResponse(ctx, http.StatusForbidden, err.Error())
		}
		if errors.Is(err, entity.ErrVideoNotFound) {
			return errorResponse(ctx, http.StatusNotFound, err.Error())
		}
		return errorResponse(ctx, http.StatusInternalServerError, "failed to delete video")
	}

	return ctx.Status(http.StatusOK).JSON(fiber.Map{
		"message": "video deleted successfully",
	})
}

// @Summary      Retry a failed video transcode
// @Description  Re-queues a NATS transcode job for a video whose previous transcode attempt failed, reusing the already-uploaded raw file
// @Tags         Studio
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Video ID"
// @Success      200 {object} response.VideoResponse
// @Failure      403 {object} response.Error
// @Failure      404 {object} response.Error
// @Failure      409 {object} response.Error
// @Failure      500 {object} response.Error
// @Router       /v1/studio/videos/{id}/retry [post]
func (r *V1) retryTranscode(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	userID := getUserID(ctx)

	resDTO, err := r.vd.RetryTranscode(ctx.UserContext(), userID, id)
	if err != nil {
		r.l.Error(err, "restapi - v1 - retryTranscode")
		if errors.Is(err, entity.ErrVideoForbidden) {
			return errorResponse(ctx, http.StatusForbidden, err.Error())
		}
		if errors.Is(err, entity.ErrVideoNotFound) {
			return errorResponse(ctx, http.StatusNotFound, err.Error())
		}
		if errors.Is(err, entity.ErrVideoNotFailed) {
			return errorResponse(ctx, http.StatusConflict, err.Error())
		}
		if errors.Is(err, entity.ErrTranscodeQueueUnavailable) {
			return errorResponse(ctx, http.StatusServiceUnavailable, entity.ErrTranscodeQueueUnavailable.Error())
		}
		return errorResponse(ctx, http.StatusInternalServerError, "failed to retry transcode")
	}

	return ctx.Status(http.StatusOK).JSON(resDTO)
}

// @Summary      Get video subtitles
// @Description  Bilingual (English/Vietnamese) WebVTT cues for a video, if any were uploaded
// @Tags         Videos
// @Produce      json
// @Param        id path string true "Video ID"
// @Success      200 {object} response.Response[response.VideoSubtitlesResponse]
// @Router       /v1/videos/{id}/subtitles [get]
func (r *V1) getVideoSubtitles(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	if ok, err := r.videoViewable(ctx, id); !ok {
		return err
	}

	resDTO, err := r.sub.GetSubtitles(ctx.UserContext(), id)
	if err != nil {
		r.l.Error(err, "restapi - v1 - getVideoSubtitles")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to load subtitles")
	}

	return ctx.Status(http.StatusOK).JSON(response.Response[response.VideoSubtitlesResponse]{
		Success: true,
		Data:    resDTO,
	})
}

// @Summary      Upload video subtitles
// @Description  Parse and store a bilingual (English required, Vietnamese optional) WebVTT subtitle track, replacing any previous one
// @Tags         Studio
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Video ID"
// @Param        request body request.UploadSubtitles true "WebVTT content"
// @Success      200 {object} map[string]interface{}
// @Failure      400 {object} response.Error
// @Failure      403 {object} response.Error
// @Router       /v1/studio/videos/{id}/subtitles [post]
func (r *V1) uploadVideoSubtitles(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	userID := getUserID(ctx)

	var body request.UploadSubtitles
	if err := ctx.BodyParser(&body); err != nil {
		return errorResponse(ctx, http.StatusBadRequest, "invalid request body")
	}
	if err := r.v.Struct(body); err != nil {
		return errorResponse(ctx, http.StatusBadRequest, err.Error())
	}

	if err := r.sub.UploadSubtitles(ctx.UserContext(), userID, id, body.VTTEnglish, body.VTTVietnamese); err != nil {
		r.l.Error(err, "restapi - v1 - uploadVideoSubtitles")
		if errors.Is(err, entity.ErrVideoForbidden) {
			return errorResponse(ctx, http.StatusForbidden, err.Error())
		}
		if errors.Is(err, entity.ErrVideoNotFound) {
			return errorResponse(ctx, http.StatusNotFound, err.Error())
		}
		return errorResponse(ctx, http.StatusBadRequest, err.Error())
	}

	return ctx.Status(http.StatusOK).JSON(fiber.Map{"success": true})
}
