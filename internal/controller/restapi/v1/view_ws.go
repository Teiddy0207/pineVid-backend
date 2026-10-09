package v1

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/events"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

const (
	wsPingInterval = 30 * time.Second
	wsWriteTimeout = 10 * time.Second
)

// wsViewsUpgrade runs before the WebSocket upgrade of
// GET /v1/ws/videos/:id/views: it rejects non-WebSocket requests and videos
// the caller may not watch, and loads the count sent right after connecting.
func (r *V1) wsViewsUpgrade(ctx *fiber.Ctx) error {
	if !websocket.IsWebSocketUpgrade(ctx) {
		return errorResponse(ctx, http.StatusUpgradeRequired, "WebSocket upgrade required")
	}
	if r.viewHub == nil {
		return errorResponse(ctx, http.StatusServiceUnavailable, "live view counts unavailable")
	}

	// ctx.Params is backed by fasthttp's pooled request buffer, which later
	// requests overwrite; the id outlives this handler (it keys the hub
	// subscription), so it must be copied.
	videoID := strings.Clone(ctx.Params("id"))
	total, err := r.vd.CurrentViews(ctx.UserContext(), videoID, getUserID(ctx))
	if err != nil {
		if errors.Is(err, entity.ErrVideoNotFound) {
			return errorResponse(ctx, http.StatusNotFound, "video not found")
		}
		r.l.Error(err, "restapi - v1 - wsViewsUpgrade")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to load view count")
	}

	ctx.Locals("video_id", videoID)
	ctx.Locals("initial_views", total)
	return ctx.Next()
}

// wsViews streams the video's view total (events.ViewUpdate JSON frames):
// once on connect, then whenever it changes.
func (r *V1) wsViews(conn *websocket.Conn) {
	videoID, _ := conn.Locals("video_id").(string)
	initial, _ := conn.Locals("initial_views").(int64)

	updates, unsubscribe := r.viewHub.Subscribe(videoID)
	defer unsubscribe()

	// The client sends nothing; reading only detects that it went away.
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	send := func(u events.ViewUpdate) bool {
		_ = conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
		return conn.WriteJSON(u) == nil
	}

	if !send(events.ViewUpdate{VideoID: videoID, TotalViews: initial, TS: time.Now().UnixMilli()}) {
		return
	}

	ping := time.NewTicker(wsPingInterval)
	defer ping.Stop()

	for {
		select {
		case <-closed:
			return
		case u, ok := <-updates:
			if !ok || !send(u) {
				return
			}
		case <-ping.C:
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
