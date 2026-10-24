package middleware

import (
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

func TestRequestLoggerDoesNotConsumeStreamingBody(t *testing.T) {
	app := fiber.New()
	raw := &fasthttp.RequestCtx{}
	ctx := app.AcquireCtx(raw)
	defer app.ReleaseCtx(ctx)
	ctx.Response().SetBodyStream(strings.NewReader("data: {\"viewers_count\":3}\n\n"), -1)
	message := buildRequestMessage(ctx)
	if !ctx.Response().IsBodyStream() {
		t.Fatal("logger drained the SSE stream")
	}
	if !strings.HasSuffix(message, " -") {
		t.Fatalf("expected unknown streaming length: %s", message)
	}
}
