package middleware

import (
	"net/http"
	"strings"

	"github.com/evrone/go-clean-template/pkg/jwt"
	"github.com/gofiber/fiber/v2"
)

const _bearerParts = 2

type errorResponse struct {
	Error string `json:"error"`
}

// Auth returns a JWT authentication middleware for Fiber.
func Auth(jwtManager *jwt.Manager) func(*fiber.Ctx) error {
	return func(ctx *fiber.Ctx) error {
		header := ctx.Get("Authorization")
		if header == "" {
			return ctx.Status(http.StatusUnauthorized).JSON(errorResponse{Error: "missing authorization header"})
		}

		parts := strings.SplitN(header, " ", _bearerParts)
		if len(parts) != _bearerParts || parts[0] != "Bearer" {
			return ctx.Status(http.StatusUnauthorized).JSON(errorResponse{Error: "invalid authorization header format"})
		}

		userID, err := jwtManager.ParseToken(parts[1])
		if err != nil {
			return ctx.Status(http.StatusUnauthorized).JSON(errorResponse{Error: "invalid or expired token"})
		}

		ctx.Locals("userID", userID)

		return ctx.Next()
	}
}

// OptionalAuth tries to resolve the caller's userID from a Bearer token if one
// is present, but never blocks the request — it's for public routes that want
// to behave differently for logged-in callers (e.g. attributing a watch-history
// entry) while still working for anonymous ones. Any missing/malformed/invalid
// token is treated the same as "no token": ctx.Locals("userID") is simply left unset.
func OptionalAuth(jwtManager *jwt.Manager) func(*fiber.Ctx) error {
	return func(ctx *fiber.Ctx) error {
		header := ctx.Get("Authorization")
		if header == "" {
			return ctx.Next()
		}

		parts := strings.SplitN(header, " ", _bearerParts)
		if len(parts) != _bearerParts || parts[0] != "Bearer" {
			return ctx.Next()
		}

		userID, err := jwtManager.ParseToken(parts[1])
		if err != nil {
			return ctx.Next()
		}

		ctx.Locals("userID", userID)

		return ctx.Next()
	}
}
