// Package transcodejob queues transcode jobs (docs/message-contract.md §2.1).
// Both manual uploads and livestream DVR replays go through Enqueue, so every
// job gets a job_id, a queued_at timestamp and a rollback when the queue is
// unreachable.
package transcodejob

import (
	"context"
	"fmt"
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
	if err := e.pub.PublishTranscodeJob(ctx, job); err != nil {
		if rbErr := e.repo.RevertQueued(ctx, videoID, jobID, from, e.now()); rbErr != nil {
			return "", false, fmt.Errorf("%w (publish: %v; rollback: %v)", ErrQueueUnavailable, err, rbErr)
		}
		return "", false, fmt.Errorf("%w (publish: %v)", ErrQueueUnavailable, err)
	}

	return jobID, true, nil
}
