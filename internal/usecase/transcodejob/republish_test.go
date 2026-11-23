package transcodejob_test

import (
	"context"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase/transcodejob"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// trackingRepo adds the PublishTracker side of the Postgres repo.
type trackingRepo struct {
	fakeRepo
	published []string // jobIDs passed to MarkPublished
	stuck     []entity.Video
	before    time.Time
}

func (r *trackingRepo) MarkPublished(_ context.Context, _, jobID string, _ time.Time) error {
	r.published = append(r.published, jobID)
	return nil
}

func (r *trackingRepo) ListUnpublishedJobs(_ context.Context, before time.Time, _ int) ([]entity.Video, error) {
	r.before = before
	return r.stuck, nil
}

func TestEnqueue_MarksPublishedAfterPublish(t *testing.T) {
	t.Parallel()
	r := &trackingRepo{fakeRepo: fakeRepo{markOK: true}}
	pub := &fakePublisher{}

	jobID, queued, err := transcodejob.New(r, pub, nil).Enqueue(context.Background(), "v1", "k", entity.VideoStatusPending)
	require.NoError(t, err)
	assert.True(t, queued)
	assert.Equal(t, []string{jobID}, r.published)
}

func TestRepublishUnpublished_SameJobID(t *testing.T) {
	t.Parallel()
	q := time.Now().Add(-time.Minute)
	r := &trackingRepo{stuck: []entity.Video{{ID: "v1", CurrentJobID: "job-1", RawS3Key: "raw-uploads/v1/raw.mp4", QueuedAt: &q}}}
	pub := &fakePublisher{}

	n, err := transcodejob.New(r, pub, nil).RepublishUnpublished(context.Background(), 30*time.Second)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	require.Len(t, pub.jobs, 1)
	assert.Equal(t, "job-1", pub.jobs[0].JobID, "same job_id → same Nats-Msg-Id → JetStream dedupes")
	assert.Equal(t, "v1", pub.jobs[0].VideoID)
	assert.Equal(t, "raw-uploads/v1/raw.mp4", pub.jobs[0].SourceKey)
	assert.Equal(t, []string{"job-1"}, r.published)
	assert.WithinDuration(t, time.Now().Add(-30*time.Second), r.before, 2*time.Second)
}

func TestRepublishUnpublished_NoTrackerIsNoop(t *testing.T) {
	t.Parallel()
	n, err := transcodejob.New(&fakeRepo{}, &fakePublisher{}, nil).RepublishUnpublished(context.Background(), time.Second)
	require.NoError(t, err)
	assert.Zero(t, n)
}
