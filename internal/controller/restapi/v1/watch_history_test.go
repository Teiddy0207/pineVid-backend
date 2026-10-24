package v1

import (
	"context"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"
)

type historyViewStub struct {
	usecase.Video
	recorded bool
	err      error
}

func (s historyViewStub) RecordView(context.Context, string, string, string, string, string) (bool, int64, error) {
	return s.recorded, 4, s.err
}

type historyRecorder struct {
	usecase.History
	calls       int
	user, video string
	seconds     int
}

func (s *historyRecorder) RecordWatch(_ context.Context, user, video string, seconds int) error {
	s.calls++
	s.user = user
	s.video = video
	s.seconds = seconds
	return nil
}
func TestValidatedViewRecordsHistoryIncludingDeduplicatedViews(t *testing.T) {
	for _, tc := range []struct {
		name, user string
		recorded   bool
		err        error
		calls      int
		status     int
	}{
		{"new view", "user-1", true, nil, 1, 200},
		{"deduplicated view", "user-1", false, nil, 1, 200},
		{"anonymous", "", true, nil, 0, 200},
		{"invalid ticket", "user-1", false, entity.ErrViewTicketInvalid, 0, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			history := &historyRecorder{}
			r := &V1{vd: historyViewStub{recorded: tc.recorded, err: tc.err}, hs: history}
			app := fiber.New()
			app.Post("/videos/:id/views", func(c *fiber.Ctx) error { c.Locals("userID", tc.user); return r.recordVideoView(c) })
			req := httptest.NewRequest("POST", "/videos/video-1/views", strings.NewReader(`{"ticket":"ticket"}`))
			req.Header.Set("Content-Type", "application/json")
			res, err := app.Test(req)
			require.NoError(t, err)
			defer res.Body.Close()
			require.Equal(t, tc.status, res.StatusCode)
			require.Equal(t, tc.calls, history.calls)
			if history.calls > 0 {
				require.Equal(t, "user-1", history.user)
				require.Equal(t, "video-1", history.video)
				require.Equal(t, 10, history.seconds)
			}
		})
	}
}
