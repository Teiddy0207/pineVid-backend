package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/middleware"
	"github.com/evrone/go-clean-template/pkg/webhooksig"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequireWebhookSignature(t *testing.T) {
	t.Parallel()

	const secret = "s3cret"
	now := time.Unix(1_700_000_000, 0)

	app := fiber.New()
	app.Post("/hook", middleware.RequireWebhookSignature(secret, func() time.Time { return now }), func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	})

	body := []byte(`{"job_id":"j1"}`)
	send := func(ts int64, sig string, payload []byte) int {
		req := httptest.NewRequest(http.MethodPost, "/hook", bytes.NewReader(payload))
		req.Header.Set(webhooksig.HeaderTimestamp, strconv.FormatInt(ts, 10))
		req.Header.Set(webhooksig.HeaderSignature, sig)
		resp, err := app.Test(req)
		require.NoError(t, err)
		return resp.StatusCode
	}

	good := webhooksig.Sign(secret, now.Unix(), body)
	assert.Equal(t, http.StatusOK, send(now.Unix(), good, body))
	assert.Equal(t, http.StatusUnauthorized, send(now.Unix(), good, []byte(`{"job_id":"j2"}`)), "tampered body")
	assert.Equal(t, http.StatusUnauthorized, send(now.Unix(), webhooksig.Sign("wrong", now.Unix(), body), body), "wrong secret")

	old := now.Add(-10 * time.Minute).Unix()
	assert.Equal(t, http.StatusUnauthorized, send(old, webhooksig.Sign(secret, old, body), body), "replayed request")
}
