package middleware

import (
	"errors"
	"net/http"
	"time"

	"github.com/evrone/go-clean-template/pkg/webhooksig"
	"github.com/gofiber/fiber/v2"
)

// RequireWebhookSignature verifies the HMAC-SHA256 signature of a signed
// service webhook (docs/message-contract.md §4.1) over the raw request body.
// now is injectable for tests; nil means time.Now.
func RequireWebhookSignature(secret string, now func() time.Time) fiber.Handler {
	if now == nil {
		now = time.Now
	}
	return func(ctx *fiber.Ctx) error {
		err := webhooksig.Verify(secret,
			ctx.Get(webhooksig.HeaderTimestamp),
			ctx.Get(webhooksig.HeaderSignature),
			ctx.Body(),
			now(),
		)
		if err != nil {
			msg := "invalid webhook signature"
			if errors.Is(err, webhooksig.ErrExpired) {
				msg = "webhook timestamp expired"
			}
			return ctx.Status(http.StatusUnauthorized).JSON(errorResponse{Error: msg})
		}
		return ctx.Next()
	}
}
