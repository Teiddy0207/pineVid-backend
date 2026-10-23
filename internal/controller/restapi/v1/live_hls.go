package v1

import (
	"errors"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/gofiber/fiber/v2"
)

func (r *V1) liveMaster(ctx *fiber.Ctx) error {
	playlist, err := r.ls.LiveMaster(ctx.UserContext(), ctx.Params("id"))
	ctx.Set("Cache-Control", "no-store")
	if err != nil {
		if errors.Is(err, entity.ErrStreamBanned) {
			return ctx.SendStatus(fiber.StatusForbidden)
		}
		if errors.Is(err, entity.ErrLivestreamNotFound) {
			return ctx.SendStatus(fiber.StatusNotFound)
		}
		ctx.Set("Retry-After", "2")
		return ctx.Status(fiber.StatusServiceUnavailable).SendString("Live stream is starting or unavailable")
	}
	ctx.Set("Content-Type", "application/vnd.apple.mpegurl")
	return ctx.SendString(playlist)
}
