package livesession_test

import (
	"context"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo/persistent/livesession"
	"github.com/evrone/go-clean-template/internal/repo/persistent/pgtest"
	"github.com/evrone/go-clean-template/internal/repo/persistent/video"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestReplayInheritsLiveViewsExactlyOnce(t *testing.T) {
	pg := pgtest.New(t)
	ctx := context.Background()
	r := livesession.New(pg)
	stream, session, videoID, owner := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	_, err := pg.Pool.Exec(ctx, `INSERT INTO livestreams (id,user_id,stream_key) VALUES ($1,$2,$3)`, stream, owner, uuid.NewString())
	require.NoError(t, err)
	now := time.Now().UTC()
	require.NoError(t, r.Store(ctx, &entity.LiveSession{ID: session, LivestreamID: stream, StartedAt: now, CreatedAt: now}))
	require.NoError(t, video.New(pg).Store(ctx, &entity.Video{ID: videoID, UserID: owner, Status: entity.VideoStatusComplete, Visibility: entity.VideoVisibilityPrivate, Views: 4, CreatedAt: now, UpdatedAt: now}))
	require.NoError(t, r.RecordViewer(ctx, session, "alice"))
	require.NoError(t, r.RecordViewer(ctx, session, "alice"))
	require.NoError(t, r.RecordViewer(ctx, session, "bob"))
	require.NoError(t, r.SetReplayVideo(ctx, session, videoID))
	require.NoError(t, r.SetReplayVideo(ctx, session, videoID))
	require.NoError(t, r.RecordViewer(ctx, session, "alice"))
	require.NoError(t, r.RecordViewer(ctx, session, "charlie"))
	var views int64
	require.NoError(t, pg.Pool.QueryRow(ctx, "SELECT views FROM videos WHERE id=$1", videoID).Scan(&views))
	require.EqualValues(t, 7, views) // 4 replay views + 3 distinct live viewers
}
