package livepresence_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/internal/repo/persistent/livepresence"
	redispkg "github.com/evrone/go-clean-template/pkg/redis"
	"github.com/stretchr/testify/require"
)

func setup(t *testing.T) (repo.LivePresence, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	mr.SetTime(time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC))
	client, err := redispkg.New(mr.Addr(), "", 0)
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })
	return livepresence.New(client), mr
}
func TestTabsDeduplicateAndExpireIndependently(t *testing.T) {
	store, mr := setup(t)
	ctx := context.Background()
	apply := func(action, tab, who string, want int64) {
		t.Helper()
		n, err := store.Apply(ctx, "broadcast", action, tab, who)
		require.NoError(t, err)
		require.Equal(t, want, n)
	}
	apply("heartbeat", "tab1", "user1", 1)
	apply("heartbeat", "tab2", "user1", 1)
	apply("heartbeat", "tab3", "user2", 2)
	apply("leave", "tab1", "wrong-user", 2)
	apply("leave", "tab1", "user1", 2)
	mr.SetTime(time.Date(2026, 10, 24, 0, 0, 30, 0, time.UTC))
	apply("heartbeat", "tab2", "user1", 2)
	mr.SetTime(time.Date(2026, 10, 24, 0, 0, 46, 0, time.UTC))
	apply("count", "", "", 1) // user2 expired without any new heartbeat
	apply("leave", "tab2", "user1", 0)
}
func TestBroadcastIsolationAndIdentityChange(t *testing.T) {
	store, _ := setup(t)
	ctx := context.Background()
	n, err := store.Apply(ctx, "old", "heartbeat", "tab", "guest")
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
	n, err = store.Apply(ctx, "new", "count", "", "")
	require.NoError(t, err)
	require.Zero(t, n)
	n, err = store.Apply(ctx, "old", "heartbeat", "tab", "signed-user")
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
	n, err = store.Apply(ctx, "old", "leave", "tab", "guest")
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
}
func TestConcurrentTabsOnePerson(t *testing.T) {
	store, _ := setup(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := store.Apply(context.Background(), "live", "heartbeat", string(rune('a'+i)), "same-user")
			require.NoError(t, err)
		}(i)
	}
	wg.Wait()
	n, err := store.Apply(context.Background(), "live", "count", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
}
