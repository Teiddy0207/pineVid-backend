package watchprogress

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type fakeDB struct {
	fail bool
	args []any
}

func (d *fakeDB) Exec(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
	d.args = args
	if d.fail {
		return pgconn.CommandTag{}, errors.New("database unavailable")
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}
func (*fakeDB) QueryRow(context.Context, string, ...any) pgx.Row { panic("unexpected database read") }
func setup(t *testing.T) (*Service, *miniredis.Miniredis, *fakeDB) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	db := &fakeDB{}
	return &Service{Redis: client, PG: db}, mr, db
}
func TestQueueAndCacheTrackLatestPositionIncludingBackwardSeek(t *testing.T) {
	s, _, _ := setup(t)
	ctx := context.Background()
	require.NoError(t, s.Save(ctx, "u", "v", Progress{Position: 320}))
	first, err := s.Load(ctx, "u", "v")
	require.NoError(t, err)
	require.NoError(t, s.Save(ctx, "u", "v", Progress{Position: 180}))
	latest, err := s.Load(ctx, "u", "v")
	require.NoError(t, err)
	require.Equal(t, 180.0, latest.Position)
	require.NotEqual(t, first.Version, latest.Version)
	require.NoError(t, s.Save(ctx, "other", "v", Progress{Position: 42, Completed: true}))
	latest, err = s.Load(ctx, "u", "v")
	require.NoError(t, err)
	require.Equal(t, 180.0, latest.Position)
	other, err := s.Load(ctx, "other", "v")
	require.NoError(t, err)
	require.True(t, other.Completed)
	require.EqualValues(t, 3, s.Redis.XLen(ctx, Stream).Val())
}
func TestDatabaseFailureRetainsPendingEventForRecovery(t *testing.T) {
	s, _, db := setup(t)
	ctx := context.Background()
	require.NoError(t, s.Save(ctx, "u", "v", Progress{Position: 180, Completed: true}))
	require.NoError(t, s.Redis.XGroupCreate(ctx, Stream, group, "0").Err())
	batches, err := s.Redis.XReadGroup(ctx, &redis.XReadGroupArgs{Group: group, Consumer: "crashed-worker", Streams: []string{Stream, ">"}}).Result()
	require.NoError(t, err)
	m := batches[0].Messages[0]
	db.fail = true
	require.Error(t, s.handle(ctx, m))
	require.EqualValues(t, 1, s.Redis.XPending(ctx, Stream, group).Val().Count)
	require.EqualValues(t, 1, s.Redis.XLen(ctx, Stream).Val())
	claimed, _, err := s.Redis.XAutoClaim(ctx, &redis.XAutoClaimArgs{Stream: Stream, Group: group, Consumer: "replacement", Start: "0-0", MinIdle: 0}).Result()
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	db.fail = false
	require.NoError(t, s.handle(ctx, claimed[0]))
	require.Equal(t, 180.0, db.args[2])
	require.Equal(t, true, db.args[3])
	require.Equal(t, m.ID, db.args[4])
	require.Zero(t, s.Redis.XPending(ctx, Stream, group).Val().Count)
	require.Zero(t, s.Redis.XLen(ctx, Stream).Val())
	// Redelivery after a commit is harmless; the SQL version guard makes writes idempotent.
	require.NoError(t, s.handle(ctx, m))
}
func TestQueueFailureDoesNotReportSuccess(t *testing.T) {
	s, mr, _ := setup(t)
	mr.SetError("ERR unavailable")
	require.Error(t, s.Save(context.Background(), "u", "v", Progress{Position: 20}))
}
