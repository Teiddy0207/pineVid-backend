package v1

import (
	"context"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/events"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/logger"
	pkgnats "github.com/evrone/go-clean-template/pkg/nats"
	fasthttpws "github.com/fasthttp/websocket"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

// fakeVideoUC implements only CurrentViews; other usecase.Video methods
// panic on the nil embedded interface.
type fakeVideoUC struct {
	usecase.Video
	views map[string]int64
}

func (f fakeVideoUC) CurrentViews(_ context.Context, videoID, _ string) (int64, error) {
	n, ok := f.views[videoID]
	if !ok {
		return 0, entity.ErrVideoNotFound
	}
	return n, nil
}

func startWSServer(t *testing.T, hub *events.ViewHub) string {
	t.Helper()
	r := &V1{vd: fakeVideoUC{views: map[string]int64{"v1": 41}}, viewHub: hub, l: logger.New("error")}
	app := fiber.New()
	app.Get("/v1/ws/videos/:id/views", r.wsViewsUpgrade, websocket.New(r.wsViews))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = app.Listener(ln) }()
	t.Cleanup(func() { _ = app.Shutdown() })
	return "ws://" + ln.Addr().String()
}

func readFrame(t *testing.T, c *fasthttpws.Conn) events.ViewUpdate {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	var u events.ViewUpdate
	if err := c.ReadJSON(&u); err != nil {
		t.Fatalf("read frame: %v", err)
	}
	return u
}

func TestWSViews_InitialCountThenLiveUpdates(t *testing.T) {
	hub := events.NewViewHub(10 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)
	base := startWSServer(t, hub)

	c, _, err := fasthttpws.DefaultDialer.Dial(base+"/v1/ws/videos/v1/views", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if u := readFrame(t, c); u.VideoID != "v1" || u.TotalViews != 41 {
		t.Fatalf("initial frame = %+v", u)
	}

	start := time.Now()
	hub.Publish(events.ViewUpdate{VideoID: "v1", TotalViews: 42, TS: start.UnixMilli()})
	if u := readFrame(t, c); u.TotalViews != 42 {
		t.Fatalf("update frame = %+v", u)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("update took %v", d)
	}
}

// The production wiring: a view published on NATS by the same Publisher that
// subscribes to it reaches the WebSocket client.
func TestWSViews_ThroughNATS(t *testing.T) {
	url := os.Getenv("TEST_NATS_URL")
	if url == "" {
		t.Skip("TEST_NATS_URL not set")
	}
	t.Setenv("NATS_URL", url)
	pub, err := pkgnats.NewPublisher()
	if err != nil {
		t.Fatal(err)
	}
	defer pub.Close()

	hub := events.NewViewHub(250 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)
	if _, err := pub.SubscribeViewUpdated(func(ev pkgnats.ViewUpdated) {
		hub.Publish(events.ViewUpdate{VideoID: ev.VideoID, TotalViews: ev.TotalViews, TS: ev.TS})
	}); err != nil {
		t.Fatal(err)
	}

	base := startWSServer(t, hub)
	c, _, err := fasthttpws.DefaultDialer.Dial(base+"/v1/ws/videos/v1/views", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	readFrame(t, c) // initial

	for i := int64(42); i < 45; i++ {
		if err := pub.PublishViewUpdated(pkgnats.ViewUpdated{VideoID: "v1", TotalViews: i, TS: time.Now().UnixMilli()}); err != nil {
			t.Fatal(err)
		}
		if u := readFrame(t, c); u.TotalViews != i {
			t.Fatalf("got %+v, want total %d", u, i)
		}
	}
}

// Regression: ctx.Params returns a string backed by fasthttp's reusable
// request buffer. Kept as the hub key it was silently rewritten by later
// requests, so updates for the real video id found no watcher.
func TestWSViews_VideoIDSurvivesLaterRequests(t *testing.T) {
	hub := events.NewViewHub(10 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	r := &V1{vd: fakeVideoUC{views: map[string]int64{"aaaaaaaaaa": 1}}, viewHub: hub, l: logger.New("error")}
	app := fiber.New()
	app.Get("/v1/ws/videos/:id/views", r.wsViewsUpgrade, websocket.New(r.wsViews))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = app.Listener(ln) }()
	t.Cleanup(func() { _ = app.Shutdown() })
	base := "ws://" + ln.Addr().String()

	c, _, err := fasthttpws.DefaultDialer.Dial(base+"/v1/ws/videos/aaaaaaaaaa/views", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	readFrame(t, c)

	// Plain requests of the same shape reuse the pooled request buffers.
	for range 50 {
		resp, err := http.Get("http://" + ln.Addr().String() + "/v1/ws/videos/zzzzzzzzzz/views")
		if err == nil {
			resp.Body.Close()
		}
	}

	hub.Publish(events.ViewUpdate{VideoID: "aaaaaaaaaa", TotalViews: 2, TS: time.Now().UnixMilli()})
	if u := readFrame(t, c); u.VideoID != "aaaaaaaaaa" || u.TotalViews != 2 {
		t.Fatalf("got %+v", u)
	}
}

func TestWSViews_RejectsUnknownVideoAndPlainHTTP(t *testing.T) {
	base := startWSServer(t, events.NewViewHub(time.Hour))

	_, resp, err := fasthttpws.DefaultDialer.Dial(base+"/v1/ws/videos/nope/views", nil)
	if err == nil || resp == nil || resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("unknown video: err=%v resp=%v", err, resp)
	}
}
