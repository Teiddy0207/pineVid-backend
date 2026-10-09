package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"time"

	"github.com/nats-io/nats.go"
)

// Names from docs/message-contract.md §1.
const (
	StreamVideoJobs     = "VIDEO_JOBS"
	SubjectTranscode    = "video.transcode"
	SubjectTranscodeDLQ = "video.transcode.dlq"
	SubjectViewsUpdated = "video.views.updated"
	// DLQConsumer is the backend's durable consumer on SubjectTranscodeDLQ.
	DLQConsumer = "backend-dlq"
	// DuplicateWindow is how long JetStream remembers a Nats-Msg-Id.
	DuplicateWindow = 2 * time.Minute

	// legacyStream held video.transcode before the contract existed; its
	// subject overlaps VIDEO_JOBS, so it has to go before VIDEO_JOBS can exist.
	legacyStream = "PIPEVID_STREAM"
)

var ErrUnavailable = errors.New("nats: publisher unavailable")

// TranscodeJob is the video.transcode payload (contract §2.1).
type TranscodeJob struct {
	SchemaVersion int       `json:"schema_version"`
	JobID         string    `json:"job_id"`
	VideoID       string    `json:"video_id"`
	SourceKey     string    `json:"source_key"`
	Renditions    []string  `json:"renditions"`
	CreatedAt     time.Time `json:"created_at"`
}

// TranscodeDLQMessage is the video.transcode.dlq payload (contract §2.2).
type TranscodeDLQMessage struct {
	SchemaVersion int       `json:"schema_version"`
	JobID         string    `json:"job_id"`
	VideoID       string    `json:"video_id"`
	Error         string    `json:"error"`
	Attempts      int       `json:"attempts"`
	FailedAt      time.Time `json:"failed_at"`
}

// ViewUpdated is the video.views.updated payload (contract §2.3).
type ViewUpdated struct {
	VideoID    string `json:"video_id"`
	TotalViews int64  `json:"total_views"`
	TS         int64  `json:"ts"`
}

type Publisher struct {
	nc *nats.Conn
	js nats.JetStreamContext
}

func NewPublisher() (*Publisher, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://guest:guest@localhost:4222"
	}

	// No fallback to another URL: silently connecting to whatever NATS
	// listens on the default port could hand jobs to an unrelated server.
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("NATS Connect error: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("NATS JetStream error: %w", err)
	}

	if err := ensureVideoJobsStream(js); err != nil {
		nc.Close()
		return nil, err
	}

	return &Publisher{nc: nc, js: js}, nil
}

// ensureVideoJobsStream creates (or brings up to date) the VIDEO_JOBS stream
// the contract describes. The backend owns the stream; workers only bind.
func ensureVideoJobsStream(js nats.JetStreamContext) error {
	if info, err := js.StreamInfo(legacyStream); err == nil && slices.Equal(info.Config.Subjects, []string{SubjectTranscode}) {
		log.Printf("WARN nats: deleting legacy stream %s (replaced by %s)", legacyStream, StreamVideoJobs)
		if err := js.DeleteStream(legacyStream); err != nil {
			return fmt.Errorf("NATS delete legacy stream: %w", err)
		}
	}

	cfg := &nats.StreamConfig{
		Name:       StreamVideoJobs,
		Subjects:   []string{SubjectTranscode, SubjectTranscodeDLQ},
		Storage:    nats.FileStorage,
		Duplicates: DuplicateWindow,
	}

	if _, err := js.StreamInfo(StreamVideoJobs); err != nil {
		if !errors.Is(err, nats.ErrStreamNotFound) {
			return fmt.Errorf("NATS stream info: %w", err)
		}
		if _, err := js.AddStream(cfg); err != nil {
			return fmt.Errorf("NATS add stream %s: %w", StreamVideoJobs, err)
		}
		return nil
	}

	if _, err := js.UpdateStream(cfg); err != nil {
		return fmt.Errorf("NATS update stream %s: %w", StreamVideoJobs, err)
	}
	return nil
}

// PublishTranscodeJob publishes a job with Nats-Msg-Id = job_id, so a
// retried publish inside DuplicateWindow is stored only once.
func (p *Publisher) PublishTranscodeJob(ctx context.Context, job TranscodeJob) error {
	if p == nil || p.js == nil {
		return ErrUnavailable
	}

	data, err := json.Marshal(job)
	if err != nil {
		return err
	}

	_, err = p.js.Publish(SubjectTranscode, data, nats.MsgId(job.JobID), nats.Context(ctx))
	return err
}

// PublishViewUpdated announces a new view count on core NATS (not stored).
func (p *Publisher) PublishViewUpdated(ev ViewUpdated) error {
	if p == nil || p.nc == nil {
		return ErrUnavailable
	}

	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}

	return p.nc.Publish(SubjectViewsUpdated, data)
}

// SubscribeViewUpdated delivers every video.views.updated message to handler.
func (p *Publisher) SubscribeViewUpdated(handler func(ViewUpdated)) (*nats.Subscription, error) {
	if p == nil || p.nc == nil {
		return nil, ErrUnavailable
	}

	return p.nc.Subscribe(SubjectViewsUpdated, func(msg *nats.Msg) {
		var ev ViewUpdated
		if err := json.Unmarshal(msg.Data, &ev); err != nil {
			return
		}
		handler(ev)
	})
}

// dlqRetryDelay is how long a DLQ message whose handling failed waits before
// JetStream redelivers it.
const dlqRetryDelay = 5 * time.Second

// SubscribeTranscodeDLQ consumes video.transcode.dlq through the durable
// consumer DLQConsumer. A handler error redelivers the message later; a
// malformed message is terminated.
func (p *Publisher) SubscribeTranscodeDLQ(handler func(TranscodeDLQMessage) error) (*nats.Subscription, error) {
	if p == nil || p.js == nil {
		return nil, ErrUnavailable
	}

	return p.js.Subscribe(SubjectTranscodeDLQ, func(msg *nats.Msg) {
		var m TranscodeDLQMessage
		if err := json.Unmarshal(msg.Data, &m); err != nil || m.JobID == "" || m.VideoID == "" {
			_ = msg.Term()
			return
		}
		if err := handler(m); err != nil {
			_ = msg.NakWithDelay(dlqRetryDelay)
			return
		}
		_ = msg.Ack()
	}, nats.Durable(DLQConsumer), nats.ManualAck(), nats.AckWait(30*time.Second), nats.DeliverAll())
}

func (p *Publisher) Close() {
	if p != nil && p.nc != nil {
		p.nc.Close()
	}
}
