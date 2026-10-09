package entity_test

import (
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
)

func TestDecideTranscodeTransition(t *testing.T) {
	t.Parallel()

	started := time.Now()
	processing := entity.Video{Status: entity.VideoStatusProcessing, CurrentJobID: "job-1"}
	processingStarted := processing
	processingStarted.TranscodeStartedAt = &started

	tests := []struct {
		name       string
		video      entity.Video
		status     entity.TranscodeEventStatus
		jobID      string
		wantApply  bool
		wantReason string
	}{
		{"processing on a queued video", processing, entity.TranscodeEventProcessing, "job-1", true, ""},
		{"processing twice (already started)", processingStarted, entity.TranscodeEventProcessing, "job-1", false, entity.TranscodeSkipInvalidTransition},
		{"completed from processing", processingStarted, entity.TranscodeEventCompleted, "job-1", true, ""},
		{"completed without a processing event", processing, entity.TranscodeEventCompleted, "job-1", true, ""},
		{"failed from processing", processingStarted, entity.TranscodeEventFailed, "job-1", true, ""},
		{"processing after complete (backwards)", entity.Video{Status: entity.VideoStatusComplete, CurrentJobID: "job-1"}, entity.TranscodeEventProcessing, "job-1", false, entity.TranscodeSkipInvalidTransition},
		{"failed after complete", entity.Video{Status: entity.VideoStatusComplete, CurrentJobID: "job-1"}, entity.TranscodeEventFailed, "job-1", false, entity.TranscodeSkipInvalidTransition},
		{"completed after failed", entity.Video{Status: entity.VideoStatusFailed, CurrentJobID: "job-1"}, entity.TranscodeEventCompleted, "job-1", false, entity.TranscodeSkipInvalidTransition},
		{"banned video never revives", entity.Video{Status: entity.VideoStatusBanned, CurrentJobID: "job-1"}, entity.TranscodeEventCompleted, "job-1", false, entity.TranscodeSkipInvalidTransition},
		{"event from an older job", processing, entity.TranscodeEventCompleted, "job-0", false, entity.TranscodeSkipStaleJob},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			apply, reason := entity.DecideTranscodeTransition(tc.video, entity.TranscodeEvent{JobID: tc.jobID, Status: tc.status})
			assert.Equal(t, tc.wantApply, apply)
			assert.Equal(t, tc.wantReason, reason)
		})
	}
}

func TestValidateTranscodeEvent(t *testing.T) {
	t.Parallel()

	valid := entity.TranscodeEvent{
		IdempotencyKey: "job-1:COMPLETED",
		JobID:          "job-1",
		VideoID:        "video-1",
		Status:         entity.TranscodeEventCompleted,
		HLSMasterURL:   "/hls-streams/videos/video-1/master.m3u8",
	}
	assert.NoError(t, entity.ValidateTranscodeEvent(valid))

	keyForOtherStatus := valid
	keyForOtherStatus.IdempotencyKey = "job-1:PROCESSING"
	assert.ErrorIs(t, entity.ValidateTranscodeEvent(keyForOtherStatus), entity.ErrInvalidIdempotencyKey)

	keyForOtherJob := valid
	keyForOtherJob.IdempotencyKey = "job-2:COMPLETED"
	assert.ErrorIs(t, entity.ValidateTranscodeEvent(keyForOtherJob), entity.ErrInvalidIdempotencyKey)

	noURL := valid
	noURL.HLSMasterURL = ""
	assert.Error(t, entity.ValidateTranscodeEvent(noURL))

	unknown := valid
	unknown.Status = "DONE"
	unknown.IdempotencyKey = "job-1:DONE"
	assert.Error(t, entity.ValidateTranscodeEvent(unknown))
}
