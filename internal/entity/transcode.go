package entity

import (
	"errors"
	"fmt"
	"strings"
)

// TranscodeSchemaVersion is the message-contract version this backend speaks
// (docs/message-contract.md). Bump it whenever a payload shape changes.
const TranscodeSchemaVersion = 1

// TranscodeEventStatus is the status carried by a transcode webhook / DLQ
// message. It is also the suffix of the event's idempotency key.
type TranscodeEventStatus string

const (
	TranscodeEventProcessing TranscodeEventStatus = "PROCESSING"
	TranscodeEventCompleted  TranscodeEventStatus = "COMPLETED"
	TranscodeEventFailed     TranscodeEventStatus = "FAILED"
)

// Reasons a transcode event is acknowledged but not applied (a no-op).
const (
	TranscodeSkipDuplicate         = "duplicate"
	TranscodeSkipStaleJob          = "stale_job"
	TranscodeSkipInvalidTransition = "invalid_transition"
)

var (
	ErrInvalidIdempotencyKey = errors.New("Idempotency-Key must be <job_id>:<STATUS> and match the body")
	// ErrTranscodeQueueUnavailable means a job could not be handed to the
	// queue; the video was put back in its previous status so the caller
	// can retry.
	ErrTranscodeQueueUnavailable = errors.New("transcode queue unavailable, please retry")
)

// TranscodeEvent is one state change reported for a transcode job.
type TranscodeEvent struct {
	IdempotencyKey string
	JobID          string
	VideoID        string
	Status         TranscodeEventStatus
	HLSMasterURL   string
	ErrorMessage   string
}

// TranscodeEventResult reports what ApplyTranscodeEvent did. Before is the
// video as it was just before the event (zero value for a duplicate).
type TranscodeEventResult struct {
	Applied bool
	Reason  string
	Before  Video
}

// TranscodeIdempotencyKey builds the key the worker sends for an event.
func TranscodeIdempotencyKey(jobID string, status TranscodeEventStatus) string {
	return jobID + ":" + string(status)
}

// ValidateTranscodeEvent checks the event is well formed and that its
// idempotency key is exactly <job_id>:<status> for the same job and status.
func ValidateTranscodeEvent(ev TranscodeEvent) error {
	switch ev.Status {
	case TranscodeEventProcessing, TranscodeEventCompleted, TranscodeEventFailed:
	default:
		return fmt.Errorf("unknown transcode status %q", ev.Status)
	}
	if ev.JobID == "" || ev.VideoID == "" {
		return errors.New("job_id and video_id are required")
	}
	if ev.Status == TranscodeEventCompleted && strings.TrimSpace(ev.HLSMasterURL) == "" {
		return errors.New("hls_master_url is required for COMPLETED")
	}
	if ev.IdempotencyKey != TranscodeIdempotencyKey(ev.JobID, ev.Status) {
		return ErrInvalidIdempotencyKey
	}
	return nil
}

// DecideTranscodeTransition applies the one-way state machine from the
// message contract (§4.3) to the video's current state:
//
//	PROCESSING  only while processing and not yet started
//	COMPLETED   only from processing
//	FAILED      only from processing
//
// An event for a job other than the video's current job is stale. Anything
// else (a backwards move, or a video already complete/failed/banned) is an
// invalid transition. Both are acknowledged as no-ops by the caller.
func DecideTranscodeTransition(v Video, ev TranscodeEvent) (apply bool, reason string) {
	if v.CurrentJobID != ev.JobID {
		return false, TranscodeSkipStaleJob
	}
	if v.Status != VideoStatusProcessing {
		return false, TranscodeSkipInvalidTransition
	}
	if ev.Status == TranscodeEventProcessing && v.TranscodeStartedAt != nil {
		return false, TranscodeSkipInvalidTransition
	}
	return true, ""
}
