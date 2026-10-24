package watchprogress

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// Uses a temporary table on a dedicated connection; no application rows are changed.
func TestPostgresRejectsStaleAndDuplicateEvents(t *testing.T) {
	url := os.Getenv("TEST_WATCH_PROGRESS_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_WATCH_PROGRESS_DATABASE_URL for PostgreSQL integration test")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	require.NoError(t, err)
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, `CREATE TEMP TABLE watch_history(user_id TEXT,video_id TEXT,watch_seconds INT NOT NULL DEFAULT 0,last_watched_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,PRIMARY KEY(user_id,video_id))`)
	require.NoError(t, err)
	migration, err := os.ReadFile("../../../migrations/20261024000002_watch_progress.up.sql")
	require.NoError(t, err)
	_, err = conn.Exec(ctx, string(migration))
	require.NoError(t, err)
	migration, err = os.ReadFile("../../../migrations/20261024000003_watch_progress_duration.up.sql")
	require.NoError(t, err)
	_, err = conn.Exec(ctx, string(migration))
	require.NoError(t, err)
	s, _, _ := setup(t)
	s.PG = conn
	require.NoError(t, s.Save(ctx, "u", "v", Progress{Position: 320}))
	require.NoError(t, s.Save(ctx, "u", "v", Progress{Position: 180, Duration: 400}))
	events, err := s.Redis.XRange(ctx, Stream, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, events, 2)
	// A backwards seek is newer even though its position is smaller.
	require.NoError(t, s.handle(ctx, events[1]))
	require.NoError(t, s.handle(ctx, events[0]))
	require.NoError(t, s.handle(ctx, events[1]))
	require.NoError(t, s.Redis.Del(ctx, key("u", "v")).Err())
	p, err := s.Load(ctx, "u", "v")
	require.NoError(t, err)
	require.Equal(t, 180.0, p.Position)
	require.Equal(t, 400.0, p.Duration)
	require.Equal(t, events[1].ID, p.Version)
	require.NoError(t, s.Save(ctx, "u", "v", Progress{Position: 400, Completed: true}))
	events, err = s.Redis.XRange(ctx, Stream, "-", "+").Result()
	require.NoError(t, err)
	require.NoError(t, s.handle(ctx, events[0]))
	require.NoError(t, s.Redis.Del(ctx, key("u", "v")).Err())
	p, err = s.Load(ctx, "u", "v")
	require.NoError(t, err)
	require.True(t, p.Completed)
	missing, err := s.Load(ctx, "new-user", "v")
	require.NoError(t, err)
	require.Zero(t, missing.Position)
}
