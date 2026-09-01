package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/evrone/go-clean-template/internal/controller/restapi/middleware"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeUserUseCase implements usecase.User with a stubbed GetUser, since
// RequireAdmin only calls GetUser — the other methods are never exercised
// here so they return zero values.
type fakeUserUseCase struct {
	getUser func(ctx context.Context, userID string) (entity.User, error)
}

func (f *fakeUserUseCase) Register(context.Context, string, string, string) (entity.User, error) {
	return entity.User{}, nil
}
func (f *fakeUserUseCase) Login(context.Context, string, string) (string, error) { return "", nil }
func (f *fakeUserUseCase) RefreshToken(context.Context, string) (response.Token, error) {
	return response.Token{}, nil
}
func (f *fakeUserUseCase) GetUser(ctx context.Context, userID string) (entity.User, error) {
	return f.getUser(ctx, userID)
}
func (f *fakeUserUseCase) UpdateUser(context.Context, string, string, string, string) (entity.User, error) {
	return entity.User{}, nil
}

func newAdminTestApp(t *testing.T, callerUserID string, u *fakeUserUseCase) *fiber.App {
	t.Helper()

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if callerUserID != "" {
			c.Locals("userID", callerUserID)
		}
		return c.Next()
	})
	app.Use(middleware.RequireAdmin(u))
	app.Get("/admin", func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	})

	return app
}

func TestRequireAdmin(t *testing.T) {
	t.Parallel()

	t.Run("missing userID", func(t *testing.T) {
		t.Parallel()

		u := &fakeUserUseCase{getUser: func(context.Context, string) (entity.User, error) {
			t.Fatal("GetUser should not be called when userID is missing")
			return entity.User{}, nil
		}}
		app := newAdminTestApp(t, "", u)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin", http.NoBody)
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("regular user forbidden", func(t *testing.T) {
		t.Parallel()

		u := &fakeUserUseCase{getUser: func(context.Context, string) (entity.User, error) {
			return entity.User{ID: "user-1", Role: entity.UserRoleUser}, nil
		}}
		app := newAdminTestApp(t, "user-1", u)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin", http.NoBody)
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("user not found forbidden", func(t *testing.T) {
		t.Parallel()

		u := &fakeUserUseCase{getUser: func(context.Context, string) (entity.User, error) {
			return entity.User{}, entity.ErrUserNotFound
		}}
		app := newAdminTestApp(t, "ghost", u)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin", http.NoBody)
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("admin allowed", func(t *testing.T) {
		t.Parallel()

		u := &fakeUserUseCase{getUser: func(context.Context, string) (entity.User, error) {
			return entity.User{ID: "admin-1", Role: entity.UserRoleAdmin}, nil
		}}
		app := newAdminTestApp(t, "admin-1", u)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin", http.NoBody)
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
