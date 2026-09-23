package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/evrone/go-clean-template/pkg/jwt"
	"github.com/gofiber/fiber/v2"
)

const _bearerParts = 2

type errorResponse struct {
	Error string `json:"error"`
}

// BanChecker reports whether an account is currently banned. Checked on every
// authenticated request so a ban takes effect immediately instead of only at
// the next login — the JWT itself stays valid until it expires. A nil
// BanChecker disables the check (used by unit tests).
type BanChecker interface {
	IsBanned(ctx context.Context, userID string) (bool, error)
}

var (
	errMissingAuthHeader = errors.New("missing authorization header")
	errInvalidAuthHeader = errors.New("invalid authorization header format")
)

// bearerToken extracts the token from "Authorization: Bearer <token>".
func bearerToken(ctx *fiber.Ctx) (string, error) {
	header := ctx.Get("Authorization")
	if header == "" {
		return "", errMissingAuthHeader
	}

	parts := strings.SplitN(header, " ", _bearerParts)
	if len(parts) != _bearerParts || parts[0] != "Bearer" {
		return "", errInvalidAuthHeader
	}

	return parts[1], nil
}

func isBanned(ctx *fiber.Ctx, bans BanChecker, userID string) bool {
	if bans == nil {
		return false
	}
	banned, err := bans.IsBanned(ctx.UserContext(), userID)
	// Fail open on lookup errors: a DB hiccup shouldn't lock every user out.
	return err == nil && banned
}

func authenticate(jwtManager *jwt.Manager, bans BanChecker, allowQueryToken bool) func(*fiber.Ctx) error {
	return func(ctx *fiber.Ctx) error {
		token, err := bearerToken(ctx)
		// Browser EventSource can't send an Authorization header, so SSE
		// endpoints accept the same JWT as a ?token= query parameter instead.
		if err != nil && allowQueryToken {
			if q := ctx.Query("token"); q != "" {
				token, err = q, nil
			}
		}
		if err != nil {
			return ctx.Status(http.StatusUnauthorized).JSON(errorResponse{Error: err.Error()})
		}

		userID, err := jwtManager.ParseToken(token)
		if err != nil {
			return ctx.Status(http.StatusUnauthorized).JSON(errorResponse{Error: "invalid or expired token"})
		}

		if isBanned(ctx, bans, userID) {
			return ctx.Status(http.StatusForbidden).JSON(errorResponse{Error: "account is banned"})
		}

		ctx.Locals("userID", userID)

		return ctx.Next()
	}
}

// Auth returns a JWT authentication middleware for Fiber.
func Auth(jwtManager *jwt.Manager, bans BanChecker) func(*fiber.Ctx) error {
	return authenticate(jwtManager, bans, false)
}

// SSEAuth is Auth for Server-Sent Events endpoints: it additionally accepts
// the JWT as a ?token= query parameter (see authenticate). The logger
// middleware skips /events/ paths, so the token never reaches the request log.
func SSEAuth(jwtManager *jwt.Manager, bans BanChecker) func(*fiber.Ctx) error {
	return authenticate(jwtManager, bans, true)
}

// OptionalAuth tries to resolve the caller's userID from a Bearer token if one
// is present, but never blocks the request — it's for public routes that want
// to behave differently for logged-in callers (e.g. attributing a watch-history
// entry) while still working for anonymous ones. Any missing/malformed/invalid
// token — or a banned account — is treated the same as "no token":
// ctx.Locals("userID") is simply left unset.
func OptionalAuth(jwtManager *jwt.Manager, bans BanChecker) func(*fiber.Ctx) error {
	return func(ctx *fiber.Ctx) error {
		token, err := bearerToken(ctx)
		if err != nil {
			return ctx.Next()
		}

		userID, err := jwtManager.ParseToken(token)
		if err != nil || isBanned(ctx, bans, userID) {
			return ctx.Next()
		}

		ctx.Locals("userID", userID)

		return ctx.Next()
	}
}
