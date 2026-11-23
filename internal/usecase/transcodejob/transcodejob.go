// Package transcodejob queues transcode jobs (docs/message-contract.md §2.1).
// Both manual uploads and livestream DVR replays go through Enqueue, so every
// job gets a job_id, a queued_at timestamp and a rollback when the queue is
// unreachable.
package transcodejob

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/pkg/nats"
	"github.com/google/uuid"
)

// ErrQueueUnavailable is entity.ErrTranscodeQueueUnavailable, re-exported for
// callers of this package.
var ErrQueueUnavailable = entity.ErrTranscodeQueueUnavailable

// DefaultRenditions is the experiment ladder fixed by the proposal (§1.5).
var DefaultRenditions = []string{"720p", "480p"}

// Publisher is the part of pkg/nats.Publisher the enqueuer needs.
type Publisher interface {
	PublishTranscodeJob(ctx context.Context, job nats.TranscodeJob) error
}

type Enqueuer struct {
	repo       repo.VideoRepo
	pub        Publisher
	renditions []string
	now        func() time.Time
}

// New returns an Enqueuer. A nil pub makes every Enqueue fail with
// ErrQueueUnavailable; empty renditions fall back to DefaultRenditions.
func New(r repo.VideoRepo, pub Publisher, renditions []string) *Enqueuer {
	if len(renditions) == 0 {
		renditions = DefaultRenditions
	}
	return &Enqueuer{repo: r, pub: pub, renditions: renditions, now: func() time.Time { return time.Now().UTC() }}
}

// Enqueue makes a new job the video's current job (only if the video is still
// in status from) and publishes it. queued is false, with no error, when the
// video had already left from — e.g. a double-clicked confirm.
func (e *Enqueuer) Enqueue(ctx context.Context, videoID, sourceKey string, from entity.VideoStatus) (jobID string, queued bool, err error) {
	if e == nil || e.pub == nil {
		return "", false, ErrQueueUnavailable
	}

	jobID = uuid.NewString()
	now := e.now()

	ok, err := e.repo.MarkQueued(ctx, videoID, jobID, from, now)
	if err != nil {
		return "", false, fmt.Errorf("transcodejob - Enqueue - MarkQueued: %w", err)
	}
	if !ok {
		return "", false, nil
	}

	job := nats.TranscodeJob{
		SchemaVersion: entity.TranscodeSchemaVersion,
		JobID:         jobID,
		VideoID:       videoID,
		SourceKey:     sourceKey,
		Renditions:    e.renditions,
		CreatedAt:     now,
	}
	faultPoint("after_mark_queued") // test-only: job marked queued, never published
	if err := e.pub.PublishTranscodeJob(ctx, job); err != nil {
		if rbErr := e.repo.RevertQueued(ctx, videoID, jobID, from, e.now()); rbErr != nil {
			return "", false, fmt.Errorf("%w (publish: %v; rollback: %v)", ErrQueueUnavailable, err, rbErr)
		}
		return "", false, fmt.Errorf("%w (publish: %v)", ErrQueueUnavailable, err)
	}
	faultPoint("after_publish") // test-only: published, the client never gets the answer
	e.markPublished(ctx, videoID, jobID)

	return jobID, true, nil
}

// PublishTracker is implemented by the Postgres video repo. It closes the gap
// between MarkQueued (a DB write) and the NATS publish, which are not one
// transaction: if the API dies in between, the video sits in processing with
// a job nobody will ever receive. Mocks need not implement it.
type PublishTracker interface {
	MarkPublished(ctx context.Context, id, jobID string, at time.Time) error
	ListUnpublishedJobs(ctx context.Context, before time.Time, limit int) ([]entity.Video, error)
}

func (e *Enqueuer) markPublished(ctx context.Context, videoID, jobID string) {
	if t, ok := e.repo.(PublishTracker); ok {
		// A failure here only means the reconcile loop republishes the same
		// job later; JetStream drops it as a duplicate (same Nats-Msg-Id).
		_ = t.MarkPublished(ctx, videoID, jobID, e.now())
	}
}

// RepublishUnpublished republishes, with the SAME job_id (and so the same
// Nats-Msg-Id), every job marked queued more than olderThan ago that was never
// confirmed published. If it was in fact published, JetStream drops the copy
// inside the duplicate window, and after it a second delivery only produces
// webhooks that are already-applied no-ops. Returns how many were republished.
func (e *Enqueuer) RepublishUnpublished(ctx context.Context, olderThan time.Duration) (int, error) {
	t, ok := e.repo.(PublishTracker)
	if !ok || e.pub == nil {
		return 0, nil
	}
	stuck, err := t.ListUnpublishedJobs(ctx, e.now().Add(-olderThan), 100)
	if err != nil {
		return 0, fmt.Errorf("transcodejob - RepublishUnpublished - list: %w", err)
	}
	n := 0
	for _, v := range stuck {
		created := e.now()
		if v.QueuedAt != nil {
			created = *v.QueuedAt
		}
		job := nats.TranscodeJob{
			SchemaVersion: entity.TranscodeSchemaVersion,
			JobID:         v.CurrentJobID,
			VideoID:       v.ID,
			SourceKey:     v.RawS3Key,
			Renditions:    e.renditions,
			CreatedAt:     created,
		}
		if err := e.pub.PublishTranscodeJob(ctx, job); err != nil {
			return n, fmt.Errorf("transcodejob - RepublishUnpublished - publish %s: %w", v.CurrentJobID, err)
		}
		_ = t.MarkPublished(ctx, v.ID, v.CurrentJobID, e.now())
		n++
	}
	return n, nil
}

// faultPoint exits the process with code 3 when TRANSCODE_ENQUEUE_FAULT names
// this point. Test-only (bench/scripts/enqueue_gap_docker.sh); unset in prod.
func faultPoint(name string) {
	if os.Getenv("TRANSCODE_ENQUEUE_FAULT") == name {
		fmt.Fprintf(os.Stderr, "TRANSCODE_ENQUEUE_FAULT=%s: exiting\n", name)
		os.Exit(3)
	}
}
