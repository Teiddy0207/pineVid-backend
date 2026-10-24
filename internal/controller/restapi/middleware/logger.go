package middleware

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/gofiber/fiber/v2"
)

// redactedQueryParams are credentials that some endpoints accept in the query
// string (SSE ?token=, SRS hook ?secret=) and must never reach the logs.
var redactedQueryParams = []string{"token", "secret"}

func redactQuery(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.RawQuery == "" {
		return rawURL
	}

	q := u.Query()
	changed := false
	for _, key := range redactedQueryParams {
		if q.Has(key) {
			q.Set(key, "REDACTED")
			changed = true
		}
	}
	if !changed {
		return rawURL
	}

	u.RawQuery = q.Encode()
	return u.String()
}

func buildRequestMessage(ctx *fiber.Ctx) string {
	var result strings.Builder

	result.WriteString(ctx.IP())
	result.WriteString(" - ")
	result.WriteString(ctx.Method())
	result.WriteString(" ")
	result.WriteString(redactQuery(ctx.OriginalURL()))
	result.WriteString(" - ")
	result.WriteString(strconv.Itoa(ctx.Response().StatusCode()))
	result.WriteString(" ")
	// Reading Body drains a streaming response and prevents SSE from reaching
	// the browser until the stream closes. Log an unknown size instead.
	if ctx.Response().IsBodyStream() {
		result.WriteString("-")
	} else {
		result.WriteString(strconv.Itoa(len(ctx.Response().Body())))
	}

	return result.String()
}

func Logger(l logger.Interface) func(c *fiber.Ctx) error {
	return func(ctx *fiber.Ctx) error {
		if strings.Contains(ctx.Path(), "/events/") {
			return ctx.Next()
		}

		err := ctx.Next()

		l.Info("%s", buildRequestMessage(ctx))

		return err
	}
}
