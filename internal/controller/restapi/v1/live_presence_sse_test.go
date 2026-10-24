package v1

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/evrone/go-clean-template/internal/controller/restapi/middleware"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/internal/repo/persistent/livepresence"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/logger"
	redispkg "github.com/evrone/go-clean-template/pkg/redis"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

type presenceIntegration struct {
	usecase.Livestream
	store repo.LivePresence
}

func (s *presenceIntegration) ViewerPresence(ctx context.Context, id, action, tab, who, broadcast string) (response.LiveViewersResponse, error) {
	n, err := s.store.Apply(ctx, "live:current", action, tab, who)
	return response.LiveViewersResponse{StreamID: id, Broadcast: "live:current", ViewersCount: n}, err
}

func TestLiveViewersSSEUpdatesAndExpiresWithoutNewArrival(t *testing.T) {
	mr := miniredis.RunT(t)
	now := time.Now()
	mr.SetTime(now)
	client, err := redispkg.New(mr.Addr(), "", 0)
	require.NoError(t, err)
	defer client.Close()
	r := &V1{ls: &presenceIntegration{store: livepresence.New(client)}}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(middleware.Logger(logger.New("error")))
	app.Post("/live/:id/presence", r.liveHeartbeat)
	app.Get("/live/:id/viewers", r.liveViewersEvents)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go app.Listener(listener)
	defer app.ShutdownWithTimeout(time.Second)
	base := "http://" + listener.Addr().String()
	httpClient := &http.Client{Timeout: 20 * time.Second}
	heartbeat := func(tab, guest string) {
		t.Helper()
		body := `{"tab_id":"` + tab + `","viewer_id":"` + guest + `"}`
		res, err := httpClient.Post(base+"/live/room/presence", "application/json", strings.NewReader(body))
		require.NoError(t, err)
		defer res.Body.Close()
		require.Equal(t, 200, res.StatusCode)
	}
	heartbeat("550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440001")
	stream, err := httpClient.Get(base + "/live/room/viewers")
	require.NoError(t, err)
	defer stream.Body.Close()
	require.Equal(t, 200, stream.StatusCode)
	scanner := bufio.NewScanner(stream.Body)
	nextCount := func(want int64) {
		t.Helper()
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var value response.LiveViewersResponse
			require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &value))
			require.Equal(t, want, value.ViewersCount)
			return
		}
		t.Fatalf("SSE closed: %v", scanner.Err())
	}
	nextCount(1)
	heartbeat("550e8400-e29b-41d4-a716-446655440002", "550e8400-e29b-41d4-a716-446655440003")
	nextCount(2)
	mr.SetTime(now.Add(46 * time.Second))
	nextCount(0)
}
