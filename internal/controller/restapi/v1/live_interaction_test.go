package v1

import (
	"context"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"
)

type liveInteractionStub struct {
	usecase.Livestream
	usecase.Like
	err error
}

func (s liveInteractionStub) SendChatMessage(context.Context, string, string, string, string, string) (response.ChatMessageResponse, error) {
	return response.ChatMessageResponse{}, s.err
}
func (s liveInteractionStub) HeartStream(context.Context, string, string) (response.HeartResponse, error) {
	return response.HeartResponse{}, s.err
}

type liveUserStub struct{ usecase.User }

func (liveUserStub) GetUser(context.Context, string) (entity.User, error) {
	return entity.User{ID: "user", Username: "User"}, nil
}
func TestLiveInteractionRoomErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{{"missing", entity.ErrLivestreamNotFound, 404}, {"offline", entity.ErrLivestreamNotLive, 409}} {
		for _, kind := range []string{"chat", "heart"} {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				stub := liveInteractionStub{err: tc.err}
				r := &V1{ls: stub, lk: stub, u: liveUserStub{}, l: logger.New("error")}
				app := fiber.New()
				app.Post("/live/:id", func(c *fiber.Ctx) error {
					c.Locals("userID", "user")
					if kind == "chat" {
						return r.sendChatMessage(c)
					}
					return r.heartStream(c)
				})
				req := httptest.NewRequest("POST", "/live/room", strings.NewReader(`{"text":"hello"}`))
				req.Header.Set("Content-Type", "application/json")
				res, err := app.Test(req)
				require.NoError(t, err)
				defer res.Body.Close()
				require.Equal(t, tc.status, res.StatusCode)
			})
		}
	}
}
