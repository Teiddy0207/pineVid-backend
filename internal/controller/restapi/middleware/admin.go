package middleware

import (
	"net/http"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/gofiber/fiber/v2"
)

// RequireAdmin returns a middleware that only allows callers whose account has
// entity.UserRoleAdmin. It must run after Auth so ctx.Locals("userID") is
// already populated. Role is looked up from the database on every call
// (rather than embedded in the JWT) so that revoking admin access takes
// effect immediately instead of waiting for the token to expire.
func RequireAdmin(u usecase.User) func(*fiber.Ctx) error {
	return func(ctx *fiber.Ctx) error {
		userID, ok := ctx.Locals("userID").(string)
		if !ok || userID == "" {
			return ctx.Status(http.StatusUnauthorized).JSON(errorResponse{Error: "missing authorization header"})
		}

		user, err := u.GetUser(ctx.UserContext(), userID)
		if err != nil || user.Role != entity.UserRoleAdmin {
			return ctx.Status(http.StatusForbidden).JSON(errorResponse{Error: "admin access required"})
		}

		return ctx.Next()
	}
}
