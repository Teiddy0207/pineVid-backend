package middleware

import (
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

// RateLimiter builds per-route rate limits. Each call to Limit creates an
// independent sliding-window bucket keyed by the authenticated userID when
// present (so users behind one NAT don't share a quota), else by client IP.
// Counters live in process memory, which is fine for the single backend
// instance PipeVid runs today.
type RateLimiter struct {
	enabled bool
}

func NewRateLimiter(enabled bool) *RateLimiter {
	return &RateLimiter{enabled: enabled}
}

// Limit allows at most max requests per window for one caller. name
// namespaces the bucket so two limits on different routes never share counts.
func (rl *RateLimiter) Limit(name string, max int, window time.Duration) fiber.Handler {
	if !rl.enabled {
		return func(ctx *fiber.Ctx) error { return ctx.Next() }
	}

	return limiter.New(limiter.Config{
		Max:               max,
		Expiration:        window,
		LimiterMiddleware: limiter.SlidingWindow{},
		KeyGenerator: func(ctx *fiber.Ctx) string {
			if userID, ok := ctx.Locals("userID").(string); ok && userID != "" {
				return name + ":u:" + userID
			}
			return name + ":ip:" + ctx.IP()
		},
		LimitReached: func(ctx *fiber.Ctx) error {
			return ctx.Status(http.StatusTooManyRequests).JSON(errorResponse{Error: "too many requests, please slow down"})
		},
	})
}
