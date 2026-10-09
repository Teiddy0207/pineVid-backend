package nats_test

import (
	"context"
	"os"
	"testing"
	"time"

	pkgnats "github.com/evrone/go-clean-template/pkg/nats"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests need a JetStream-enabled server they may freely modify, e.g.
//
//	docker run --rm -d -p 54222:4222 nats:2.14-alpine -js
//	TEST_NATS_URL=nats://localhost:54222 go test ./pkg/nats/
//
// Never point TEST_NATS_URL at a shared server: the tests delete streams.
func testServer(t *testing.T) (*nats.Conn, nats.JetStreamContext) {
	t.Helper()
	url := os.Getenv("TEST_NATS_URL")
	if url == "" {
		t.Skip("TEST_NATS_URL not set; skipping NATS-backed test")
	}
	t.Setenv("NATS_URL", url)

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)

	// Start every test from a clean server.
	for _, name := range []string{pkgnats.StreamVideoJobs, "PIPEVID_STREAM"} {
		_ = js.DeleteStream(name)
	}
	return nc, js
}

func newPublisher(t *testing.T) *pkgnats.Publisher {
	t.Helper()
	p, err := pkgnats.NewPublisher()
	require.NoError(t, err)
	t.Cleanup(p.Close)
	return p
}

func TestNewPublisher_CreatesContractStream(t *testing.T) {
	_, js := testServer(t)
	newPublisher(t)

	info, err := js.StreamInfo(pkgnats.StreamVideoJobs)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"video.transcode", "video.transcode.dlq"}, info.Config.Subjects)
	assert.Equal(t, pkgnats.DuplicateWindow, info.Config.Duplicates)
	assert.Equal(t, nats.FileStorage, info.Config.Storage)
}

func TestNewPublisher_ReplacesLegacyStream(t *testing.T) {
	_, js := testServer(t)
	_, err := js.AddStream(&nats.StreamConfig{Name: "PIPEVID_STREAM", Subjects: []string{"video.transcode"}})
	require.NoError(t, err)

	newPublisher(t)

	_, err = js.StreamInfo("PIPEVID_STREAM")
	assert.ErrorIs(t, err, nats.ErrStreamNotFound)
	_, err = js.StreamInfo(pkgnats.StreamVideoJobs)
	assert.NoError(t, err)
}

// Publishing the same job twice (e.g. a client retry) stores it once,
// because Nats-Msg-Id = job_id.
func TestPublishTranscodeJob_DeduplicatesByJobID(t *testing.T) {
	_, js := testServer(t)
	p := newPublisher(t)

	job := pkgnats.TranscodeJob{SchemaVersion: 1, JobID: "job-1", VideoID: "v1", SourceKey: "k", Renditions: []string{"720p", "480p"}, CreatedAt: time.Now().UTC()}
	require.NoError(t, p.PublishTranscodeJob(context.Background(), job))
	require.NoError(t, p.PublishTranscodeJob(context.Background(), job))
	job.JobID = "job-2"
	require.NoError(t, p.PublishTranscodeJob(context.Background(), job))

	info, err := js.StreamInfo(pkgnats.StreamVideoJobs)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), info.State.Msgs)

	msg, err := js.GetMsg(pkgnats.StreamVideoJobs, 1)
	require.NoError(t, err)
	assert.Equal(t, "job-1", msg.Header.Get(nats.MsgIdHdr))
	assert.JSONEq(t, `{"schema_version":1,"job_id":"job-1","video_id":"v1","source_key":"k","renditions":["720p","480p"],"created_at":"`+job.CreatedAt.Format(time.RFC3339Nano)+`"}`, string(msg.Data))
}

func TestSubscribeTranscodeDLQ_DeliversAndRetries(t *testing.T) {
	_, js := testServer(t)
	p := newPublisher(t)

	got := make(chan pkgnats.TranscodeDLQMessage, 4)
	calls := 0
	sub, err := p.SubscribeTranscodeDLQ(func(m pkgnats.TranscodeDLQMessage) error {
		calls++
		got <- m
		if calls == 1 {
			return assert.AnError // first attempt fails → redelivered
		}
		return nil
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	_, err = js.Publish(pkgnats.SubjectTranscodeDLQ, []byte(`{"schema_version":1,"job_id":"job-9","video_id":"v9","error":"boom","attempts":3}`))
	require.NoError(t, err)

	for i := range 2 {
		select {
		case m := <-got:
			assert.Equal(t, "job-9", m.JobID)
			assert.Equal(t, "boom", m.Error)
		case <-time.After(15 * time.Second):
			t.Fatalf("delivery %d did not arrive", i+1)
		}
	}
}

func TestPublishViewUpdated_RoundTrip(t *testing.T) {
	testServer(t)
	p := newPublisher(t)

	got := make(chan pkgnats.ViewUpdated, 1)
	sub, err := p.SubscribeViewUpdated(func(ev pkgnats.ViewUpdated) { got <- ev })
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	require.NoError(t, p.PublishViewUpdated(pkgnats.ViewUpdated{VideoID: "v1", TotalViews: 42, TS: 1}))
	select {
	case ev := <-got:
		assert.Equal(t, int64(42), ev.TotalViews)
	case <-time.After(5 * time.Second):
		t.Fatal("view update not received")
	}
}

func TestNilPublisherIsUnavailable(t *testing.T) {
	var p *pkgnats.Publisher
	assert.ErrorIs(t, p.PublishTranscodeJob(context.Background(), pkgnats.TranscodeJob{}), pkgnats.ErrUnavailable)
	assert.ErrorIs(t, p.PublishViewUpdated(pkgnats.ViewUpdated{}), pkgnats.ErrUnavailable)
}
