package v1

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/internal/usecase/watchprogress"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

type progressVideoStub struct{ usecase.Video }

func (progressVideoStub) EnsureViewable(context.Context, string, string) error { return nil }

type progressStub struct {
	usecase.History
	calls int
	user  string
	p     watchprogress.Progress
}

func (s *progressStub) LoadProgress(context.Context, string, string) (watchprogress.Progress, error) {
	return watchprogress.Progress{Position: 180}, nil
}
func (s *progressStub) SaveProgress(_ context.Context, user, video string, p watchprogress.Progress) error {
	s.calls++
	s.user = user
	s.p = p
	return nil
}
func TestProgressValidationAndAuthenticatedIdentity(t *testing.T) {
	for _, tc := range []struct {
		body   string
		status int
	}{
		{`{"position_seconds":180,"user_id":"someone-else"}`, 202},
		{`{"position_seconds":-1}`, 400},
		{`{"position_seconds":604801}`, 400},
		{`{"position_seconds":"bad"}`, 400},
		{`{`, 400},
	} {
		t.Run(tc.body, func(t *testing.T) {
			s := &progressStub{}
			r := &V1{vd: progressVideoStub{}, hs: s, l: logger.New("error")}
			app := fiber.New()
			app.Put("/videos/:id/progress", func(c *fiber.Ctx) error { c.Locals("userID", "actual-user"); return r.watchProgress(c) })
			req := httptest.NewRequest("PUT", "/videos/v/progress", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			res, err := app.Test(req)
			require.NoError(t, err)
			defer res.Body.Close()
			require.Equal(t, tc.status, res.StatusCode)
			if tc.status == 202 {
				require.Equal(t, "actual-user", s.user)
				require.Equal(t, 180.0, s.p.Position)
			} else {
				require.Zero(t, s.calls)
			}
		})
	}
}
