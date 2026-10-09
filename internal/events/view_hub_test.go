package events_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func recv(t *testing.T, ch <-chan events.ViewUpdate) (events.ViewUpdate, bool) {
	t.Helper()
	select {
	case u, ok := <-ch:
		return u, ok
	case <-time.After(time.Second):
		return events.ViewUpdate{}, false
	}
}

func TestViewHub_DeliversOnlyToThatVideo(t *testing.T) {
	t.Parallel()
	h := events.NewViewHub(time.Hour)
	a, unsubA := h.Subscribe("A")
	defer unsubA()
	b, unsubB := h.Subscribe("B")
	defer unsubB()

	h.Publish(events.ViewUpdate{VideoID: "A", TotalViews: 5, TS: 1})
	h.Flush()

	u, ok := recv(t, a)
	require.True(t, ok)
	assert.Equal(t, int64(5), u.TotalViews)
	select {
	case <-b:
		t.Fatal("video B's watcher got video A's update")
	default:
	}
}

// A burst of updates between two flushes reaches the client as one frame
// with the highest total.
func TestViewHub_CoalescesBursts(t *testing.T) {
	t.Parallel()
	h := events.NewViewHub(time.Hour)
	ch, unsub := h.Subscribe("A")
	defer unsub()

	for i := int64(1); i <= 1000; i++ {
		h.Publish(events.ViewUpdate{VideoID: "A", TotalViews: i, TS: i})
	}
	h.Publish(events.ViewUpdate{VideoID: "A", TotalViews: 400}) // late, smaller: ignored
	h.Flush()

	u, _ := recv(t, ch)
	assert.Equal(t, int64(1000), u.TotalViews)
	h.Flush() // nothing new
	select {
	case extra := <-ch:
		t.Fatalf("unexpected second frame %+v", extra)
	default:
	}
}

// A slow client that didn't read the previous frame gets the newest one, and
// Flush never blocks on it.
func TestViewHub_SlowClientGetsNewest(t *testing.T) {
	t.Parallel()
	h := events.NewViewHub(time.Hour)
	ch, unsub := h.Subscribe("A")
	defer unsub()

	h.Publish(events.ViewUpdate{VideoID: "A", TotalViews: 1})
	h.Flush()
	h.Publish(events.ViewUpdate{VideoID: "A", TotalViews: 2})
	h.Flush()

	u, _ := recv(t, ch)
	assert.Equal(t, int64(2), u.TotalViews)
}

func TestViewHub_UnwatchedVideosAreIgnoredAndUnsubscribeCloses(t *testing.T) {
	t.Parallel()
	h := events.NewViewHub(time.Hour)
	h.Publish(events.ViewUpdate{VideoID: "nobody-watching", TotalViews: 9})
	h.Flush()

	ch, unsub := h.Subscribe("A")
	unsub()
	unsub() // idempotent
	_, ok := <-ch
	assert.False(t, ok, "channel is closed after unsubscribe")
}

func TestViewHub_RunFlushesPeriodically(t *testing.T) {
	t.Parallel()
	h := events.NewViewHub(20 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	ch, unsub := h.Subscribe("A")
	defer unsub()
	start := time.Now()
	h.Publish(events.ViewUpdate{VideoID: "A", TotalViews: 3})

	u, ok := recv(t, ch)
	require.True(t, ok)
	assert.Equal(t, int64(3), u.TotalViews)
	assert.Less(t, time.Since(start), 500*time.Millisecond)
}

func TestViewHub_ConcurrentUse(t *testing.T) {
	t.Parallel()
	h := events.NewViewHub(time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			ch, unsub := h.Subscribe("A")
			time.Sleep(5 * time.Millisecond)
			unsub()
			for range ch {
			}
		}()
		go func() {
			defer wg.Done()
			h.Publish(events.ViewUpdate{VideoID: "A", TotalViews: int64(i + 1)})
		}()
	}
	wg.Wait()
}
