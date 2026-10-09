package view_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/evrone/go-clean-template/internal/repo/persistent/view"
	redispkg "github.com/evrone/go-clean-template/pkg/redis"
	"github.com/stretchr/testify/require"
)

func newTestViewRepo(t *testing.T) *view.Repo {
	t.Helper()

	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	rdb, err := redispkg.New(mr.Addr(), "", 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rdb.Close() })

	return view.New(nil, rdb)
}

// TestRecordView_DedupWithin60Seconds pins down the business rule of the
// proposal (§4.3): one view per person per video per 60 s, where an anonymous
// "person" is IP + User-Agent.
func TestRecordView_DedupWithin60Seconds(t *testing.T) {
	t.Parallel()

	repo := newTestViewRepo(t)
	ctx := context.Background()

	recorded, views, err := repo.RecordView(ctx, "video-1", "", "1.2.3.4", "Firefox/130")
	require.NoError(t, err)
	require.True(t, recorded)
	require.Equal(t, int64(1), views)

	// Same IP + User-Agent again immediately: deduped, count unchanged.
	recorded, views, err = repo.RecordView(ctx, "video-1", "", "1.2.3.4", "Firefox/130")
	require.NoError(t, err)
	require.False(t, recorded)
	require.Equal(t, int64(1), views)

	// Another browser on the same IP: a distinct viewer.
	recorded, views, err = repo.RecordView(ctx, "video-1", "", "1.2.3.4", "Chrome/129")
	require.NoError(t, err)
	require.True(t, recorded)
	require.Equal(t, int64(2), views)

	// Another video: counted separately.
	recorded, _, err = repo.RecordView(ctx, "video-2", "", "1.2.3.4", "Firefox/130")
	require.NoError(t, err)
	require.True(t, recorded)
}

// A logged-in viewer is identified by the account, whatever IP or browser.
func TestRecordView_LoggedInViewerIsTheAccount(t *testing.T) {
	t.Parallel()

	repo := newTestViewRepo(t)
	ctx := context.Background()

	recorded, _, err := repo.RecordView(ctx, "video-1", "user-7", "1.2.3.4", "Firefox/130")
	require.NoError(t, err)
	require.True(t, recorded)

	// Same account from another network and browser: still the same person.
	recorded, _, err = repo.RecordView(ctx, "video-1", "user-7", "9.9.9.9", "Chrome/129")
	require.NoError(t, err)
	require.False(t, recorded)

	// Another account behind the first one's IP + browser: a different person.
	recorded, views, err := repo.RecordView(ctx, "video-1", "user-8", "1.2.3.4", "Firefox/130")
	require.NoError(t, err)
	require.True(t, recorded)
	require.Equal(t, int64(2), views)
}

func TestViewerID(t *testing.T) {
	t.Parallel()

	require.Equal(t, "u:user-7", view.ViewerID("user-7", "1.2.3.4", "UA"))
	anon := view.ViewerID("", "1.2.3.4", "UA")
	require.Equal(t, anon, view.ViewerID("", "1.2.3.4", "UA"), "stable for the same IP + UA")
	require.NotEqual(t, anon, view.ViewerID("", "1.2.3.5", "UA"))
	require.NotEqual(t, anon, view.ViewerID("", "1.2.3.4", "UA2"))
	require.NotContains(t, anon, "1.2.3.4", "the raw IP is not stored")
}

func TestRecordView_AnonymousIPCap(t *testing.T) {
	t.Parallel()

	repo := newTestViewRepo(t)
	ctx := context.Background()

	counted := 0
	for i := range 15 {
		recorded, _, err := repo.RecordView(ctx, "video-cap", "", "1.2.3.4", "UA-"+string(rune('a'+i)))
		require.NoError(t, err)
		if recorded {
			counted++
		}
	}
	require.Equal(t, 10, counted, "at most 10 anonymous views per IP per hour per video")

	repo.DisableIPCap()
	recorded, _, err := repo.RecordView(ctx, "video-cap", "", "1.2.3.4", "UA-extra")
	require.NoError(t, err)
	require.True(t, recorded, "BENCHMARK_MODE lifts the cap")
}

func TestRecordView_AllowsRecountAfterDedupWindowExpires(t *testing.T) {
	t.Parallel()

	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	rdb, err := redispkg.New(mr.Addr(), "", 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rdb.Close() })

	repo := view.New(nil, rdb)
	ctx := context.Background()

	recorded, _, err := repo.RecordView(ctx, "video-2", "", "5.6.7.8", "UA-Z")
	require.NoError(t, err)
	require.True(t, recorded)

	// Still within the 60s window: deduped.
	recorded, _, err = repo.RecordView(ctx, "video-2", "", "5.6.7.8", "UA-Z")
	require.NoError(t, err)
	require.False(t, recorded)

	// Advance miniredis's clock past the 60s TTL.
	mr.FastForward(61 * time.Second)

	recorded, views, err := repo.RecordView(ctx, "video-2", "", "5.6.7.8", "UA-Z")
	require.NoError(t, err)
	require.True(t, recorded)
	require.Equal(t, int64(2), views)
}
