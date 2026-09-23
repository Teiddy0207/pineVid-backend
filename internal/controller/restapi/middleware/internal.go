package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

// InternalTokenHeader carries the shared webhook secret on calls from our own
// services (transcode worker callback + heartbeat).
const InternalTokenHeader = "X-Internal-Token"

// RequireInternalSecret guards service-to-service webhooks (transcode
// callback, worker heartbeat, SRS http_hooks) that must never be callable by
// end users. The secret is accepted from the X-Internal-Token header or, for
// SRS — whose http_hooks can't send custom headers — a ?secret= query
// parameter baked into the hook URL in srs/live.conf.
func RequireInternalSecret(secret string) func(*fiber.Ctx) error {
	return func(ctx *fiber.Ctx) error {
		provided := ctx.Get(InternalTokenHeader)
		if provided == "" {
			provided = ctx.Query("secret")
		}

		if secret == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) != 1 {
			return ctx.Status(http.StatusUnauthorized).JSON(errorResponse{Error: "invalid internal token"})
		}

		return ctx.Next()
	}
}
