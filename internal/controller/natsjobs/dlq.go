// Package natsjobs consumes the JetStream subjects the backend receives
// (docs/message-contract.md §1): today, video.transcode.dlq.
package natsjobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/events"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/evrone/go-clean-template/pkg/nats"
)

// TranscodeEventHandler is the part of usecase.Video the DLQ consumer needs.
type TranscodeEventHandler interface {
	HandleTranscodeEvent(ctx context.Context, ev entity.TranscodeEvent) (entity.TranscodeEventResult, error)
}

const handleTimeout = 10 * time.Second

// DLQHandler turns a dead-lettered job into a FAILED event for its video,
// through the same idempotent path as the webhook (key <job_id>:FAILED), and
// tells the owner's Studio over SSE. Returning an error redelivers the
// message later; a message for a deleted video is dropped.
func DLQHandler(vd TranscodeEventHandler, hub *events.Hub, l logger.Interface) func(nats.TranscodeDLQMessage) error {
	return func(m nats.TranscodeDLQMessage) error {
		ev := entity.TranscodeEvent{
			IdempotencyKey: entity.TranscodeIdempotencyKey(m.JobID, entity.TranscodeEventFailed),
			JobID:          m.JobID,
			VideoID:        m.VideoID,
			Status:         entity.TranscodeEventFailed,
			ErrorMessage:   m.Error,
		}

		ctx, cancel := context.WithTimeout(context.Background(), handleTimeout)
		defer cancel()

		res, err := vd.HandleTranscodeEvent(ctx, ev)
		if errors.Is(err, entity.ErrVideoNotFound) {
			l.Warn("natsjobs - DLQ: video %s no longer exists, dropping job %s", m.VideoID, m.JobID)
			return nil
		}
		if err != nil {
			l.Error(fmt.Errorf("natsjobs - DLQ - HandleTranscodeEvent: %w", err))
			return err
		}

		if res.Applied && hub != nil {
			hub.Broadcast(events.VideoEvent{
				UserID:  res.Before.UserID,
				VideoID: m.VideoID,
				Status:  string(entity.VideoStatusFailed),
			})
		}
		return nil
	}
}
