package v1

import (
	"context"
	"github.com/evrone/go-clean-template/internal/usecase/watchprogress"
	"math"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

const maxHistoryLimit = 50

// @Summary      Get current user's watch history
// @Description  Paginated list of videos the current user has watched, most recent first
// @Tags         History
// @Produce      json
// @Security     BearerAuth
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Page limit" default(10)
// @Success      200 {object} response.PageResponse[response.VideoResponse]
// @Failure      401 {object} response.Error
// @Failure      500 {object} response.Error
// @Router       /v1/user/history [get]
func (r *V1) getWatchHistory(ctx *fiber.Ctx) error {
	userID, ok := ctx.Locals("userID").(string)
	if !ok || userID == "" {
		return errorResponse(ctx, http.StatusUnauthorized, "unauthorized")
	}

	page := ctx.QueryInt("page", 1)
	limit := ctx.QueryInt("limit", 10)
	if limit > maxHistoryLimit {
		limit = maxHistoryLimit
	}

	resDTO, err := r.hs.ListHistory(ctx.UserContext(), userID, page, limit)
	if err != nil {
		r.l.Error(err, "restapi - v1 - getWatchHistory")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to load watch history")
	}

	return ctx.Status(http.StatusOK).JSON(resDTO)
}

// progressHistory keeps the optional progress capability separate from legacy history mocks.
type progressHistory interface {
	LoadProgress(context.Context, string, string) (watchprogress.Progress, error)
	SaveProgress(context.Context, string, string, watchprogress.Progress) error
}

func (r *V1) watchProgress(ctx *fiber.Ctx) error {
	user, video := getUserID(ctx), ctx.Params("id")
	if ok, err := r.videoViewable(ctx, video); !ok {
		return err
	}
	service, ok := r.hs.(progressHistory)
	if !ok {
		return errorResponse(ctx, 503, "progress unavailable")
	}
	if ctx.Method() == "GET" {
		p, err := service.LoadProgress(ctx.UserContext(), user, video)
		if err != nil {
			r.l.Error(err, "load progress")
			return errorResponse(ctx, 503, "failed to load progress")
		}
		return ctx.JSON(p)
	}
	var p watchprogress.Progress
	if err := ctx.BodyParser(&p); err != nil || math.IsNaN(p.Position) || math.IsInf(p.Position, 0) || p.Position < 0 || p.Position > 604800 || math.IsNaN(p.Duration) || math.IsInf(p.Duration, 0) || p.Duration < 0 || p.Duration > 604800 {
		return errorResponse(ctx, 400, "invalid playback position")
	}
	if err := service.SaveProgress(ctx.UserContext(), user, video, p); err != nil {
		r.l.Error(err, "save progress")
		return errorResponse(ctx, 503, "failed to queue progress")
	}
	return ctx.Status(202).JSON(fiber.Map{"queued": true})
}
