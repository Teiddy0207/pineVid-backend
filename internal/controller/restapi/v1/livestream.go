package v1

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
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
		return errorResponse(ctx, http.StatusInternalServerError, "failed to get stream")
	}

	return ctx.Status(http.StatusOK).JSON(resDTO)
}

// @Summary      Send chat message
// @Description  Send a real-time message to a live stream chat room
// @Tags         Livestream
// @Accept       json
// @Produce      json
// @Param        id path string true "Livestream ID"
// @Param        request body request.SendChatMessage true "Chat message payload"
// @Success      200 {object} response.ChatMessageResponse
// @Failure      400 {object} response.Error
// @Router       /v1/live/streams/{id}/chat [post]
func (r *V1) sendChatMessage(ctx *fiber.Ctx) error {
	id := ctx.Params("id")

	var req request.SendChatMessage
	if err := ctx.BodyParser(&req); err != nil {
		return errorResponse(ctx, http.StatusBadRequest, "invalid chat payload")
	}

	if req.Text == "" {
		return errorResponse(ctx, http.StatusBadRequest, "text cannot be empty")
	}

	resDTO, err := r.ls.SendChatMessage(ctx.UserContext(), id, req)
	if err != nil {
		r.l.Error(err, "restapi - v1 - sendChatMessage")
		return errorResponse(ctx, http.StatusBadRequest, err.Error())
	}

	return ctx.Status(http.StatusOK).JSON(resDTO)
}

// @Summary      Realtime SSE Live Chat Stream
// @Description  Stream live chat messages for a specific livestream room via SSE
// @Tags         Livestream
// @Produce      text/event-stream
// @Param        id path string true "Livestream ID"
// @Router       /v1/events/chat/{id} [get]
func (r *V1) sseChatEvents(ctx *fiber.Ctx) error {
	streamID := ctx.Params("id")

	ctx.Set("Content-Type", "text/event-stream")
	ctx.Set("Cache-Control", "no-cache")
	ctx.Set("Connection", "keep-alive")
	ctx.Set("Access-Control-Allow-Origin", "*")

	ch, unsubscribe, err := r.ls.SubscribeChat(streamID)
	if err != nil {
		r.l.Error(err, "restapi - v1 - sseChatEvents")
		return errorResponse(ctx, http.StatusInternalServerError, err.Error())
	}

	ctx.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer unsubscribe()

		// Initial connection ping
		fmt.Fprintf(w, ": connected to chat room %s\n\n", streamID)
		w.Flush()

		for msg := range ch {
			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			if err := w.Flush(); err != nil {
				return // client disconnected
			}
		}
	})

	return nil
}

// @Summary      Get streamer live stream key
// @Description  Get or create the RTMP stream key for the authenticated streamer
// @Tags         Studio
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} response.StreamKeyResponse
// @Failure      500 {object} response.Error
// @Router       /v1/studio/live/key [get]
func (r *V1) getStreamKey(ctx *fiber.Ctx) error {
	userID := getUserID(ctx)

	keyDTO, err := r.ls.GetStreamKey(ctx.UserContext(), userID)
	if err != nil {
		r.l.Error(err, "restapi - v1 - getStreamKey")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to get stream key")
	}

	return ctx.Status(http.StatusOK).JSON(keyDTO)
}

// @Summary      Reset streamer live stream key
// @Description  Generate a new RTMP stream key for the authenticated streamer
// @Tags         Studio
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} response.StreamKeyResponse
// @Failure      500 {object} response.Error
// @Router       /v1/studio/live/reset-key [post]
func (r *V1) resetStreamKey(ctx *fiber.Ctx) error {
	userID := getUserID(ctx)

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
