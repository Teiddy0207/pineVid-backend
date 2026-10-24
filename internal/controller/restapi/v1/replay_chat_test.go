package v1

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/mapper"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

type replayVideoStub struct {
	usecase.Video
	result response.VideoResponse
	err    error
}

func (s replayVideoStub) GetByID(context.Context, string, string) (response.VideoResponse, error) {
	return s.result, s.err
}

type replayChatStub struct {
	usecase.Livestream
	called      bool
	page, limit int
	session     string
}

func (s *replayChatStub) GetSessionChatHistory(_ context.Context, session string, page, limit int) (response.PageResponse[response.ChatMessageResponse], error) {
	s.called, s.session, s.page, s.limit = true, session, page, limit
	return response.PageResponse[response.ChatMessageResponse]{Success: true, Data: []response.ChatMessageResponse{}}, nil
}

func TestReplayChatRequiresViewableVideo(t *testing.T) {
	for _, tc := range []struct {
		name   string
		video  replayVideoStub
		status int
	}{
		{"hidden video", replayVideoStub{err: entity.ErrVideoNotFound}, 404},
		{"ordinary upload", replayVideoStub{}, 404},
		{"replay", replayVideoStub{result: response.VideoResponse{ReplaySessionID: "session-1"}}, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chat := &replayChatStub{}
			r := &V1{vd: tc.video, ls: chat}
			app := fiber.New()
			app.Get("/videos/:id/replay-chat", r.getVideoReplayChat)
			res, err := app.Test(httptest.NewRequest("GET", "/videos/video-1/replay-chat?page=-1&limit=999999", nil))
			require.NoError(t, err)
			defer res.Body.Close()
			require.Equal(t, tc.status, res.StatusCode)
			require.Equal(t, tc.status == 200, chat.called)
			if chat.called {
				require.Equal(t, "session-1", chat.session)
				require.Equal(t, 1, chat.page)
				require.Equal(t, 100, chat.limit)
			}
		})
	}
}

func TestStoredChatPreservesAbsoluteTimestampForReplay(t *testing.T) {
	// Keep the date and timezone across midnight; a clock-only string cannot sync replay.
	sent := time.Date(2026, 10, 25, 0, 0, 2, 0, time.FixedZone("ICT", 7*3600))
	result := mapper.ToStoredChatMessageResponse(entity.LiveChatMessage{ID: 42, CreatedAt: sent})
	data, err := json.Marshal(result)
	require.NoError(t, err)
	var decoded struct {
		ID     int64     `json:"id"`
		SentAt time.Time `json:"sent_at"`
	}
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.EqualValues(t, 42, decoded.ID)
	require.True(t, decoded.SentAt.Equal(sent))
}
