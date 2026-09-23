package middleware_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/middleware"
	"github.com/evrone/go-clean-template/pkg/jwt"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestApp(t *testing.T) (*fiber.App, *jwt.Manager) {
	t.Helper()

	jwtManager := jwt.New("test-secret", time.Hour)

	app := fiber.New()
	app.Use(middleware.Auth(jwtManager, nil))
	app.Get("/test", func(c *fiber.Ctx) error {
		userID, ok := c.Locals("userID").(string)
		if !ok {
			return c.SendStatus(http.StatusUnauthorized)
		}

		return c.SendString(userID)
	})

	return app, jwtManager
}

func TestAuthMiddleware(t *testing.T) {
	t.Parallel()

	app, jwtManager := newTestApp(t)

	validToken, err := jwtManager.GenerateToken("user-id-123")
	require.NoError(t, err)

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "missing header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "invalid format",
			authHeader:     "Basic xxx",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "invalid token",
			authHeader:     "Bearer invalid",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "valid token",
			authHeader:     "Bearer " + validToken,
			expectedStatus: http.StatusOK,
			expectedBody:   "user-id-123",
		},
	}

	for _, tc := range tests {
		localTc := tc

		t.Run(localTc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", http.NoBody)
			if localTc.authHeader != "" {
				req.Header.Set("Authorization", localTc.authHeader)
			}

			resp, err := app.Test(req)
			require.NoError(t, err)

			defer resp.Body.Close()

			assert.Equal(t, localTc.expectedStatus, resp.StatusCode)

			if localTc.expectedBody != "" {
				body, readErr := io.ReadAll(resp.Body)
				require.NoError(t, readErr)
				assert.Equal(t, localTc.expectedBody, string(body))
			}
		})
	}
}

type fakeBanChecker map[string]bool

func (f fakeBanChecker) IsBanned(_ context.Context, userID string) (bool, error) {
	return f[userID], nil
}

func echoUserID(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	return c.SendString(userID)
}

func doGet(t *testing.T, app *fiber.App, target, authHeader string) (int, string) {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp.StatusCode, string(body)
}

func TestAuthMiddleware_BannedUserRejected(t *testing.T) {
	t.Parallel()

	jwtManager := jwt.New("test-secret", time.Hour)
	app := fiber.New()
	app.Get("/test", middleware.Auth(jwtManager, fakeBanChecker{"banned-user": true}), echoUserID)

	bannedToken, err := jwtManager.GenerateToken("banned-user")
	require.NoError(t, err)
	okToken, err := jwtManager.GenerateToken("good-user")
	require.NoError(t, err)

	status, _ := doGet(t, app, "/test", "Bearer "+bannedToken)
	assert.Equal(t, http.StatusForbidden, status)

	status, body := doGet(t, app, "/test", "Bearer "+okToken)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "good-user", body)
}

func TestAuthMiddleware_QueryTokenOnlyForSSE(t *testing.T) {
	t.Parallel()

	jwtManager := jwt.New("test-secret", time.Hour)
	app := fiber.New()
	app.Get("/api", middleware.Auth(jwtManager, nil), echoUserID)
	app.Get("/events", middleware.SSEAuth(jwtManager, nil), echoUserID)

	token, err := jwtManager.GenerateToken("user-1")
	require.NoError(t, err)

	// Regular API routes never accept a token in the URL.
	status, _ := doGet(t, app, "/api?token="+token, "")
	assert.Equal(t, http.StatusUnauthorized, status)

	status, body := doGet(t, app, "/events?token="+token, "")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "user-1", body)

	status, _ = doGet(t, app, "/events?token=garbage", "")
	assert.Equal(t, http.StatusUnauthorized, status)

	// A client-chosen user_id is ignored entirely.
	status, _ = doGet(t, app, "/events?user_id=victim", "")
	assert.Equal(t, http.StatusUnauthorized, status)
}

func TestOptionalAuth_BannedUserIsAnonymous(t *testing.T) {
	t.Parallel()

	jwtManager := jwt.New("test-secret", time.Hour)
	app := fiber.New()
	app.Get("/test", middleware.OptionalAuth(jwtManager, fakeBanChecker{"banned-user": true}), echoUserID)

	token, err := jwtManager.GenerateToken("banned-user")
	require.NoError(t, err)

	status, body := doGet(t, app, "/test", "Bearer "+token)
	assert.Equal(t, http.StatusOK, status)
	assert.Empty(t, body)
}

func TestRequireInternalSecret(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Post("/hook", middleware.RequireInternalSecret("s3cret"), func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	})

	post := func(target, header string) int {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, target, http.NoBody)
		if header != "" {
			req.Header.Set(middleware.InternalTokenHeader, header)
		}
		resp, err := app.Test(req)
		require.NoError(t, err)
		resp.Body.Close()
		return resp.StatusCode
	}

	assert.Equal(t, http.StatusUnauthorized, post("/hook", ""))
	assert.Equal(t, http.StatusUnauthorized, post("/hook", "wrong"))
	assert.Equal(t, http.StatusUnauthorized, post("/hook?secret=wrong", ""))
	assert.Equal(t, http.StatusOK, post("/hook", "s3cret"))
	assert.Equal(t, http.StatusOK, post("/hook?secret=s3cret", ""))
}

func TestRequireInternalSecret_EmptySecretRejectsAll(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Post("/hook", middleware.RequireInternalSecret(""), func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/hook?secret=", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
