package v1

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	liveuc "github.com/evrone/go-clean-template/internal/usecase/livestream"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type presenceRequest struct {
	TabID     string `json:"tab_id"`
	ViewerID  string `json:"viewer_id"`
	Broadcast string `json:"broadcast"`
}

func presenceError(ctx *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, entity.ErrLivestreamNotFound):
		return ctx.SendStatus(fiber.StatusNotFound)
	case errors.Is(err, entity.ErrStreamBanned):
		return ctx.SendStatus(fiber.StatusForbidden)
	case errors.Is(err, liveuc.ErrStreamOffline), errors.Is(err, liveuc.ErrStaleBroadcast):
		return ctx.SendStatus(fiber.StatusConflict)
	default:
		return ctx.SendStatus(fiber.StatusServiceUnavailable)
	}
}
func (r *V1) liveHeartbeat(ctx *fiber.Ctx) error { return r.applyPresence(ctx, "heartbeat") }
func (r *V1) liveLeave(ctx *fiber.Ctx) error     { return r.applyPresence(ctx, "leave") }
func (r *V1) applyPresence(ctx *fiber.Ctx, action string) error {
	var body presenceRequest
	if ctx.BodyParser(&body) != nil {
		return ctx.SendStatus(fiber.StatusBadRequest)
	}
	if _, err := uuid.Parse(body.TabID); err != nil {
		return ctx.SendStatus(fiber.StatusBadRequest)
	}
	identity := "user:" + getUserID(ctx)
	if getUserID(ctx) == "" {
		if _, err := uuid.Parse(body.ViewerID); err != nil {
			return ctx.SendStatus(fiber.StatusBadRequest)
		}
		identity = "guest:" + body.ViewerID
	}
	// Redis contains opaque digests, not raw account IDs or guest identifiers.
	identity = fmt.Sprintf("%x", sha256.Sum256([]byte(identity)))
	if len(body.Broadcast) > 256 {
		return ctx.SendStatus(fiber.StatusBadRequest)
	}
	if action == "leave" && body.Broadcast == "" {
		return ctx.SendStatus(fiber.StatusBadRequest)
	}
	res, err := r.ls.ViewerPresence(ctx.UserContext(), ctx.Params("id"), action, body.TabID, identity, body.Broadcast)
	if err != nil {
		return presenceError(ctx, err)
	}
	ctx.Set("Cache-Control", "no-store")
	return ctx.JSON(res)
}

// Read-only SSE polling Redis means expiry is visible even if nobody sends
// another heartbeat. Each API replica observes the same authoritative set.
func (r *V1) liveViewersEvents(ctx *fiber.Ctx) error {
	id := string([]byte(ctx.Params("id")))
	initial, err := r.ls.ViewerPresence(ctx.UserContext(), id, "count", "", "", "")
	if err != nil {
		return presenceError(ctx, err)
	}
	ctx.Set("Content-Type", "text/event-stream")
	ctx.Set("Cache-Control", "no-cache")
	ctx.Set("X-Accel-Buffering", "no")
	ctx.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		send := func(value interface{}) bool {
			data, _ := json.Marshal(value)
			_, err := fmt.Fprintf(w, "data: %s\n\n", data)
			return err == nil && w.Flush() == nil
		}
		if !send(initial) {
			return
		}
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			readCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			result, err := r.ls.ViewerPresence(readCtx, id, "count", "", "", initial.Broadcast)
			cancel()
			if err != nil {
				if errors.Is(err, liveuc.ErrStreamOffline) || errors.Is(err, liveuc.ErrStaleBroadcast) || errors.Is(err, entity.ErrStreamBanned) || errors.Is(err, entity.ErrLivestreamNotFound) {
					send(fiber.Map{"type": "ended", "viewers_count": 0})
				} else {
					send(fiber.Map{"type": "unavailable"})
				}
				return
			}
			if !send(result) {
				return
			}
		}
	})
	return nil
}
