package nats

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/nats-io/nats.go"
)

type TranscodeJobPayload struct {
	VideoID      string `json:"video_id"`
	RawObjectKey string `json:"raw_object_key"`
	UserID       string `json:"user_id"`
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

	nc, err := nats.Connect(natsURL)
	if err != nil {
		// Fallback to connection without auth
		nc, err = nats.Connect("nats://localhost:4222")
		if err != nil {
			return nil, fmt.Errorf("NATS Connect error: %w", err)
		}
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("NATS JetStream error: %w", err)
	}

	// Ensure stream exists
	_, err = js.StreamInfo("PIPEVID_STREAM")
	if err != nil {
		_, _ = js.AddStream(&nats.StreamConfig{
			Name:     "PIPEVID_STREAM",
			Subjects: []string{"video.transcode"},
		})
	}

	return &Publisher{nc: nc, js: js}, nil
}

func (p *Publisher) PublishTranscodeJob(videoID, rawObjectKey, userID string) error {
	if p == nil || p.js == nil {
		return nil
	}

	payload := TranscodeJobPayload{
		VideoID:      videoID,
		RawObjectKey: rawObjectKey,
		UserID:       userID,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	_, err = p.js.Publish("video.transcode", data)
	return err
}

func (p *Publisher) Close() {
	if p != nil && p.nc != nil {
		p.nc.Close()
	}
}
