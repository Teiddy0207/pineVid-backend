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

// TestRecordView_DedupWithin60Seconds pins down the exact business rule from
// BUSINESS_REQUIREMENTS.md: repeat views from the same IP+device fingerprint
// within a 60-second window must not double-count, but a genuinely different
// viewer (or the same one after the window elapses) must.
func TestRecordView_DedupWithin60Seconds(t *testing.T) {
	t.Parallel()

	repo := newTestViewRepo(t)
	ctx := context.Background()

	recorded, views, err := repo.RecordView(ctx, "video-1", "1.2.3.4", "device-A")
	require.NoError(t, err)
	require.True(t, recorded)
	require.Equal(t, int64(1), views)

	// Same IP + device again immediately: deduped, count unchanged.
	recorded, views, err = repo.RecordView(ctx, "video-1", "1.2.3.4", "device-A")
	require.NoError(t, err)
	require.False(t, recorded)
	require.Equal(t, int64(1), views)

	// Different device from the same IP: a genuinely distinct viewer.
	recorded, views, err = repo.RecordView(ctx, "video-1", "1.2.3.4", "device-B")
	require.NoError(t, err)
	require.True(t, recorded)
	require.Equal(t, int64(2), views)
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

	recorded, _, err := repo.RecordView(ctx, "video-2", "5.6.7.8", "device-Z")
	require.NoError(t, err)
	require.True(t, recorded)

	// Still within the 60s window: deduped.
	recorded, _, err = repo.RecordView(ctx, "video-2", "5.6.7.8", "device-Z")
	require.NoError(t, err)
	require.False(t, recorded)

	// Advance miniredis's clock past the 60s TTL.
	mr.FastForward(61 * time.Second)

	recorded, views, err := repo.RecordView(ctx, "video-2", "5.6.7.8", "device-Z")
	require.NoError(t, err)
	require.True(t, recorded)
	require.Equal(t, int64(2), views)
}
