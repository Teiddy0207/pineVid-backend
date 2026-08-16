package v1

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

// @Summary      Get user notifications
// @Description  Get a paginated list of real-time notifications for the authenticated user
// @Tags         Notification
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Page limit" default(10)
// @Success      200 {object} response.NotificationListResponse
// @Failure      500 {object} response.Error
// @Router       /v1/notifications [get]
func (r *V1) listNotifications(ctx *fiber.Ctx) error {
	userID := getUserID(ctx)
	page := ctx.QueryInt("page", 1)
	limit := ctx.QueryInt("limit", 10)

	listDTO, err := r.notif.ListNotifications(ctx.UserContext(), userID, page, limit)
	if err != nil {
		r.l.Error(err, "restapi - v1 - listNotifications")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to get notifications")
	}

	return ctx.Status(http.StatusOK).JSON(listDTO)
}

// @Summary      Mark notification as read
// @Description  Mark a specific notification as read
// @Tags         Notification
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Notification ID"
// @Success      200 {object} map[string]bool
// @Failure      500 {object} response.Error
// @Router       /v1/notifications/{id}/read [post]
func (r *V1) markNotificationRead(ctx *fiber.Ctx) error {
	userID := getUserID(ctx)
	id := ctx.Params("id")

	if err := r.notif.MarkAsRead(ctx.UserContext(), id, userID); err != nil {
		r.l.Error(err, "restapi - v1 - markNotificationRead")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to mark notification as read")
	}

	return ctx.Status(http.StatusOK).JSON(fiber.Map{"success": true})
}

// @Summary      Realtime SSE Notification Stream
// @Description  Stream real-time notifications for the authenticated user via SSE
// @Tags         Notification
// @Produce      text/event-stream
// @Security     BearerAuth
// @Router       /v1/events/notifications [get]
func (r *V1) sseNotificationEvents(ctx *fiber.Ctx) error {
	userID := getUserID(ctx)
	if userID == "" {
		userID = ctx.Query("user_id") // Fallback for query param EventSource
	}

	ctx.Set("Content-Type", "text/event-stream")
	ctx.Set("Cache-Control", "no-cache")
	ctx.Set("Connection", "keep-alive")
	ctx.Set("Access-Control-Allow-Origin", "*")

	ch, unsubscribe, err := r.notif.SubscribeNotifications(userID)
	if err != nil {
		r.l.Error(err, "restapi - v1 - sseNotificationEvents")
		return errorResponse(ctx, http.StatusInternalServerError, err.Error())
	}

	ctx.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer unsubscribe()

		// Initial connection ping
		fmt.Fprintf(w, ": connected to notification stream for user %s\n\n", userID)
		w.Flush()

		for notif := range ch {
			data, err := json.Marshal(notif)
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
