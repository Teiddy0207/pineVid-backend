package transcodejob_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/internal/usecase/transcodejob"
	"github.com/evrone/go-clean-template/pkg/nats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRepo implements only what Enqueue touches; any other VideoRepo call
// panics on the nil embedded interface.
type fakeRepo struct {
	repo.VideoRepo
	markOK     bool
	marked     []string // jobIDs passed to MarkQueued
	reverted   []string // jobIDs passed to RevertQueued
	revertedTo entity.VideoStatus
	markedFrom entity.VideoStatus
}

func (f *fakeRepo) MarkQueued(_ context.Context, _, jobID string, from entity.VideoStatus, _ time.Time) (bool, error) {
	f.marked = append(f.marked, jobID)
	f.markedFrom = from
	return f.markOK, nil
}

func (f *fakeRepo) RevertQueued(_ context.Context, _, jobID string, to entity.VideoStatus, _ time.Time) error {
	f.reverted = append(f.reverted, jobID)
	f.revertedTo = to
	return nil
}

type fakePublisher struct {
	err  error
	jobs []nats.TranscodeJob
}

func (f *fakePublisher) PublishTranscodeJob(_ context.Context, job nats.TranscodeJob) error {
	f.jobs = append(f.jobs, job)
	return f.err
}

func TestEnqueue_PublishesContractPayload(t *testing.T) {
	t.Parallel()
	r := &fakeRepo{markOK: true}
	pub := &fakePublisher{}

	jobID, queued, err := transcodejob.New(r, pub, nil).Enqueue(context.Background(), "v1", "raw-uploads/v1/raw.mp4", entity.VideoStatusPending)
	require.NoError(t, err)
	assert.True(t, queued)
	require.Len(t, pub.jobs, 1)

	job := pub.jobs[0]
	assert.Equal(t, entity.TranscodeSchemaVersion, job.SchemaVersion)
	assert.Equal(t, jobID, job.JobID)
	assert.Equal(t, []string{jobID}, r.marked, "the published job is the one recorded on the video")
	assert.Equal(t, "v1", job.VideoID)
	assert.Equal(t, "raw-uploads/v1/raw.mp4", job.SourceKey)
	assert.Equal(t, []string{"720p", "480p"}, job.Renditions)
	assert.False(t, job.CreatedAt.IsZero())
	assert.Equal(t, entity.VideoStatusPending, r.markedFrom)
}

func TestEnqueue_PublishFailureRollsBack(t *testing.T) {
	t.Parallel()
	r := &fakeRepo{markOK: true}
	pub := &fakePublisher{err: errors.New("nats down")}

	_, queued, err := transcodejob.New(r, pub, nil).Enqueue(context.Background(), "v1", "k", entity.VideoStatusFailed)
	require.ErrorIs(t, err, transcodejob.ErrQueueUnavailable)
	assert.False(t, queued)
	assert.Equal(t, r.marked, r.reverted, "the same job is rolled back")
	assert.Equal(t, entity.VideoStatusFailed, r.revertedTo, "video goes back to where it was")
}

func TestEnqueue_NotInExpectedStatus(t *testing.T) {
	t.Parallel()
	r := &fakeRepo{markOK: false}
	pub := &fakePublisher{}

	_, queued, err := transcodejob.New(r, pub, nil).Enqueue(context.Background(), "v1", "k", entity.VideoStatusPending)
	require.NoError(t, err)
	assert.False(t, queued)
	assert.Empty(t, pub.jobs, "nothing is published for a video that already left pending")
}

func TestEnqueue_NoPublisher(t *testing.T) {
	t.Parallel()
	r := &fakeRepo{markOK: true}

	_, _, err := transcodejob.New(r, nil, nil).Enqueue(context.Background(), "v1", "k", entity.VideoStatusPending)
	require.ErrorIs(t, err, transcodejob.ErrQueueUnavailable)
	assert.Empty(t, r.marked, "the video is not touched when there is no queue")

	var nilEnqueuer *transcodejob.Enqueuer
	_, _, err = nilEnqueuer.Enqueue(context.Background(), "v1", "k", entity.VideoStatusPending)
	require.ErrorIs(t, err, transcodejob.ErrQueueUnavailable)
}

func TestEnqueue_CustomRenditions(t *testing.T) {
	t.Parallel()
	r := &fakeRepo{markOK: true}
	pub := &fakePublisher{}

	_, _, err := transcodejob.New(r, pub, []string{"1080p", "720p"}).Enqueue(context.Background(), "v1", "k", entity.VideoStatusPending)
	require.NoError(t, err)
	assert.Equal(t, []string{"1080p", "720p"}, pub.jobs[0].Renditions)
}
