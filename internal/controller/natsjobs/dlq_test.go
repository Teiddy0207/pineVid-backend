package natsjobs_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/natsjobs"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/events"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/evrone/go-clean-template/pkg/nats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeHandler struct {
	got []entity.TranscodeEvent
	res entity.TranscodeEventResult
	err error
}

func (f *fakeHandler) HandleTranscodeEvent(_ context.Context, ev entity.TranscodeEvent) (entity.TranscodeEventResult, error) {
	f.got = append(f.got, ev)
	return f.res, f.err
}

func dlqMsg() nats.TranscodeDLQMessage {
	return nats.TranscodeDLQMessage{SchemaVersion: 1, JobID: "job-1", VideoID: "v1", Error: "ffmpeg exited 1", Attempts: 3, FailedAt: time.Now()}
}

func TestDLQHandler_MarksVideoFailedAndNotifiesOwner(t *testing.T) {
	t.Parallel()
	h := &fakeHandler{res: entity.TranscodeEventResult{Applied: true, Before: entity.Video{UserID: "owner"}}}
	hub := events.NewHub()
	ch, unsub := hub.Subscribe("owner")
	defer unsub()

	require.NoError(t, natsjobs.DLQHandler(h, hub, logger.New("error"))(dlqMsg()))

	require.Len(t, h.got, 1)
	ev := h.got[0]
	assert.Equal(t, "job-1:FAILED", ev.IdempotencyKey)
	assert.Equal(t, entity.TranscodeEventFailed, ev.Status)
	assert.Equal(t, "v1", ev.VideoID)
	assert.Equal(t, "ffmpeg exited 1", ev.ErrorMessage)
	assert.NoError(t, entity.ValidateTranscodeEvent(ev))

	select {
	case got := <-ch:
		assert.Equal(t, "failed", got.Status)
		assert.Equal(t, "v1", got.VideoID)
	case <-time.After(time.Second):
		t.Fatal("owner was not notified")
	}
}

func TestDLQHandler_DuplicateIsAckedWithoutNotification(t *testing.T) {
	t.Parallel()
	h := &fakeHandler{res: entity.TranscodeEventResult{Reason: entity.TranscodeSkipDuplicate}}
	hub := events.NewHub()
	ch, unsub := hub.Subscribe("owner")
	defer unsub()

	require.NoError(t, natsjobs.DLQHandler(h, hub, logger.New("error"))(dlqMsg()))
	select {
	case <-ch:
		t.Fatal("a duplicate must not notify")
	default:
	}
}

func TestDLQHandler_ErrorsAreRetriedExceptMissingVideo(t *testing.T) {
	t.Parallel()

	down := &fakeHandler{err: errors.New("db down")}
	assert.Error(t, natsjobs.DLQHandler(down, nil, logger.New("error"))(dlqMsg()), "transient errors are redelivered")

	gone := &fakeHandler{err: entity.ErrVideoNotFound}
	assert.NoError(t, natsjobs.DLQHandler(gone, nil, logger.New("error"))(dlqMsg()), "a deleted video is dropped")
}
