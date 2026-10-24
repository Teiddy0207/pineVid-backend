package video_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/internal/repo/persistent/pgtest"
	"github.com/evrone/go-clean-template/internal/repo/persistent/video"
	"github.com/evrone/go-clean-template/pkg/postgres"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const hlsURL = "/hls-streams/videos/x/master.m3u8"

func newRepo(t *testing.T) (repo.VideoRepo, *postgres.Postgres) {
	t.Helper()
	pg := pgtest.New(t)
	return video.New(pg), pg
}

// storeQueued creates a pending video and queues jobID for it.
func storeQueued(t *testing.T, r repo.VideoRepo, jobID string) string {
	t.Helper()
	ctx := context.Background()

	now := time.Now().UTC()
	v := entity.Video{
		ID: uuid.NewString(), UserID: uuid.NewString(), Title: "t", Category: "General",
		Status: entity.VideoStatusPending, Visibility: entity.VideoVisibilityPublic,
		Duration: "00:00", CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, r.Store(ctx, &v))

	ok, err := r.MarkQueued(ctx, v.ID, jobID, entity.VideoStatusPending, now)
	require.NoError(t, err)
	require.True(t, ok)
	return v.ID
}

func event(videoID, jobID string, status entity.TranscodeEventStatus) entity.TranscodeEvent {
	ev := entity.TranscodeEvent{
		IdempotencyKey: entity.TranscodeIdempotencyKey(jobID, status),
		JobID:          jobID,
		VideoID:        videoID,
		Status:         status,
	}
	if status == entity.TranscodeEventCompleted {
		ev.HLSMasterURL = hlsURL
	}
	return ev
}

func countKeys(t *testing.T, pg *postgres.Postgres, jobID string) int {
	t.Helper()
	var n int
	require.NoError(t, pg.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM processed_webhooks WHERE job_id = $1`, jobID).Scan(&n))
	return n
}

func TestMarkQueued_OnlyFromExpectedStatus(t *testing.T) {
	t.Parallel()
	r, _ := newRepo(t)
	ctx := context.Background()

	id := storeQueued(t, r, "job-1")

	// A second confirm must not enqueue another job.
	ok, err := r.MarkQueued(ctx, id, "job-2", entity.VideoStatusPending, time.Now())
	require.NoError(t, err)
	assert.False(t, ok)

	v, err := r.GetByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, entity.VideoStatusProcessing, v.Status)
	assert.Equal(t, "job-1", v.CurrentJobID)
	assert.NotNil(t, v.QueuedAt)
	assert.Nil(t, v.TranscodeStartedAt)
}

func TestRevertQueued(t *testing.T) {
	t.Parallel()
	r, _ := newRepo(t)
	ctx := context.Background()

	id := storeQueued(t, r, "job-1")
	require.NoError(t, r.RevertQueued(ctx, id, "job-1", entity.VideoStatusPending, time.Now()))

	v, err := r.GetByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, entity.VideoStatusPending, v.Status)
}

func TestApplyTranscodeEvent_HappyPathAndDuplicates(t *testing.T) {
	t.Parallel()
	r, pg := newRepo(t)
	ctx := context.Background()

	id := storeQueued(t, r, "job-1")

	res, err := r.ApplyTranscodeEvent(ctx, event(id, "job-1", entity.TranscodeEventProcessing), time.Now())
	require.NoError(t, err)
	assert.True(t, res.Applied)

	// Redelivered PROCESSING: same key → duplicate no-op.
	res, err = r.ApplyTranscodeEvent(ctx, event(id, "job-1", entity.TranscodeEventProcessing), time.Now())
	require.NoError(t, err)
	assert.False(t, res.Applied)
	assert.Equal(t, entity.TranscodeSkipDuplicate, res.Reason)

	res, err = r.ApplyTranscodeEvent(ctx, event(id, "job-1", entity.TranscodeEventCompleted), time.Now())
	require.NoError(t, err)
	assert.True(t, res.Applied)
	assert.Equal(t, entity.VideoStatusProcessing, res.Before.Status)

	// Redelivered COMPLETED → duplicate.
	res, err = r.ApplyTranscodeEvent(ctx, event(id, "job-1", entity.TranscodeEventCompleted), time.Now())
	require.NoError(t, err)
	assert.Equal(t, entity.TranscodeSkipDuplicate, res.Reason)

	// A FAILED after COMPLETED is a backwards move → rejected.
	res, err = r.ApplyTranscodeEvent(ctx, event(id, "job-1", entity.TranscodeEventFailed), time.Now())
	require.NoError(t, err)
	assert.False(t, res.Applied)
	assert.Equal(t, entity.TranscodeSkipInvalidTransition, res.Reason)

	v, err := r.GetByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, entity.VideoStatusComplete, v.Status)
	assert.Equal(t, hlsURL, v.HLSUrl)
	require.NotNil(t, v.TranscodeStartedAt)
	require.NotNil(t, v.TranscodeCompletedAt)
	assert.False(t, v.TranscodeCompletedAt.Before(*v.TranscodeStartedAt))

	// One row per distinct key: PROCESSING, COMPLETED, FAILED.
	assert.Equal(t, 3, countKeys(t, pg, "job-1"))
}

func TestApplyTranscodeEvent_StaleJob(t *testing.T) {
	t.Parallel()
	r, _ := newRepo(t)
	ctx := context.Background()

	id := storeQueued(t, r, "job-new")

	res, err := r.ApplyTranscodeEvent(ctx, event(id, "job-old", entity.TranscodeEventCompleted), time.Now())
	require.NoError(t, err)
	assert.False(t, res.Applied)
	assert.Equal(t, entity.TranscodeSkipStaleJob, res.Reason)

	v, err := r.GetByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, entity.VideoStatusProcessing, v.Status)
}

func TestApplyTranscodeEvent_UnknownVideo(t *testing.T) {
	t.Parallel()
	r, pg := newRepo(t)

	_, err := r.ApplyTranscodeEvent(context.Background(), event(uuid.NewString(), "job-x", entity.TranscodeEventCompleted), time.Now())
	require.ErrorIs(t, err, entity.ErrVideoNotFound)
	// The transaction rolled back, so the key was not burned.
	assert.Equal(t, 0, countKeys(t, pg, "job-x"))
}

// Test scenario 2 of the proposal (§7.5): the same webhook sent many times
// concurrently is applied exactly once.
func TestApplyTranscodeEvent_ConcurrentDuplicates(t *testing.T) {
	t.Parallel()
	r, pg := newRepo(t)

	id := storeQueued(t, r, "job-1")

	const n = 10
	results := make([]entity.TranscodeEventResult, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = r.ApplyTranscodeEvent(context.Background(), event(id, "job-1", entity.TranscodeEventCompleted), time.Now())
		}()
	}
	wg.Wait()

	applied := 0
	for i := range n {
		require.NoError(t, errs[i])
		if results[i].Applied {
			applied++
		} else {
			assert.Equal(t, entity.TranscodeSkipDuplicate, results[i].Reason)
		}
	}
	assert.Equal(t, 1, applied)
	assert.Equal(t, 1, countKeys(t, pg, "job-1"))
}

// Racing COMPLETED and FAILED for the same job: exactly one wins and the
// final status matches the winner.
func TestApplyTranscodeEvent_ConcurrentConflictingEvents(t *testing.T) {
	t.Parallel()
	r, _ := newRepo(t)
	ctx := context.Background()

	id := storeQueued(t, r, "job-1")

	statuses := []entity.TranscodeEventStatus{
		entity.TranscodeEventCompleted, entity.TranscodeEventFailed,
		entity.TranscodeEventCompleted, entity.TranscodeEventFailed,
		entity.TranscodeEventCompleted, entity.TranscodeEventFailed,
	}
	results := make([]entity.TranscodeEventResult, len(statuses))
	var wg sync.WaitGroup
	for i, s := range statuses {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := r.ApplyTranscodeEvent(ctx, event(id, "job-1", s), time.Now())
			assert.NoError(t, err)
			results[i] = res
		}()
	}
	wg.Wait()

	var winner entity.TranscodeEventStatus
	applied := 0
	for i, res := range results {
		if res.Applied {
			applied++
			winner = statuses[i]
		}
	}
	require.Equal(t, 1, applied)

	v, err := r.GetByID(ctx, id)
	require.NoError(t, err)
	if winner == entity.TranscodeEventCompleted {
		assert.Equal(t, entity.VideoStatusComplete, v.Status)
	} else {
		assert.Equal(t, entity.VideoStatusFailed, v.Status)
	}
}

// An owner edit loaded before a transcode completed must not write the
// stale status back.
func TestUpdate_DoesNotOverwriteStatus(t *testing.T) {
	t.Parallel()
	r, _ := newRepo(t)
	ctx := context.Background()

	id := storeQueued(t, r, "job-1")
	stale, err := r.GetByID(ctx, id)
	require.NoError(t, err)

	_, err = r.ApplyTranscodeEvent(ctx, event(id, "job-1", entity.TranscodeEventCompleted), time.Now())
	require.NoError(t, err)

	stale.Title = "edited"
	stale.UpdatedAt = time.Now()
	require.NoError(t, r.Update(ctx, &stale))

	v, err := r.GetByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "edited", v.Title)
	assert.Equal(t, entity.VideoStatusComplete, v.Status)
	assert.Equal(t, hlsURL, v.HLSUrl)
}

func TestGetByIDIncludesReplaySession(t *testing.T) {
	r, pg := newRepo(t)
	ctx := context.Background()
	videoID := storeQueued(t, r, "replay-metadata-job")
	before, err := r.GetByID(ctx, videoID)
	require.NoError(t, err)
	require.Empty(t, before.ReplaySessionID)
	require.Nil(t, before.ReplayStartedAt)

	streamID, sessionID := uuid.NewString(), uuid.NewString()
	started := time.Date(2026, 10, 24, 16, 59, 58, 0, time.UTC)
	_, err = pg.Pool.Exec(ctx, `INSERT INTO livestreams (id, user_id, stream_key) VALUES ($1, $2, $3)`, streamID, before.UserID, uuid.NewString())
	require.NoError(t, err)
	_, err = pg.Pool.Exec(ctx, `INSERT INTO live_sessions (id, livestream_id, started_at, replay_video_id) VALUES ($1, $2, $3, $4)`, sessionID, streamID, started, videoID)
	require.NoError(t, err)
	after, err := r.GetByID(ctx, videoID)
	require.NoError(t, err)
	require.Equal(t, sessionID, after.ReplaySessionID)
	require.NotNil(t, after.ReplayStartedAt)
	require.True(t, started.Equal(*after.ReplayStartedAt))
}
