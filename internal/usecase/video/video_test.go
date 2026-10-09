package video_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase/transcodejob"
	"github.com/evrone/go-clean-template/internal/usecase/video"
	"github.com/evrone/go-clean-template/pkg/nats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fakeNotification implements usecase.Notification, recording NotifyFollowers
// calls onto a buffered channel so async goroutine-fired notifications (see
// HandleTranscodeEvent) can be observed deterministically in tests instead
// of relying on a fixed sleep.
type fakeNotification struct {
	calls chan entity.NotificationType
}

func newFakeNotification() *fakeNotification {
	return &fakeNotification{calls: make(chan entity.NotificationType, 4)}
}

func (f *fakeNotification) ListNotifications(context.Context, string, int, int) (response.NotificationListResponse, error) {
	return response.NotificationListResponse{}, nil
}
func (f *fakeNotification) MarkAsRead(context.Context, string, string) error { return nil }
func (f *fakeNotification) MarkAllAsRead(context.Context, string) error      { return nil }
func (f *fakeNotification) SubscribeNotifications(string) (<-chan response.NotificationResponse, func(), error) {
	return nil, func() {}, nil
}
func (f *fakeNotification) NotifyFollowers(_ context.Context, _, _, _ string, notifType entity.NotificationType, _, _, _ string) error {
	f.calls <- notifType
	return nil
}

func (f *fakeNotification) expectCall(t *testing.T) {
	t.Helper()
	select {
	case <-f.calls:
	case <-time.After(time.Second):
		t.Fatal("expected NotifyFollowers to be called, but it wasn't")
	}
}

func (f *fakeNotification) expectNoCall(t *testing.T) {
	t.Helper()
	select {
	case notifType := <-f.calls:
		t.Fatalf("expected NotifyFollowers not to be called, but got %v", notifType)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestCreateUpload_Visibility(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		visibility string
		want       entity.VideoVisibility
	}{
		{"defaults to public when omitted", "", entity.VideoVisibilityPublic},
		{"public", "public", entity.VideoVisibilityPublic},
		{"private", "private", entity.VideoVisibilityPrivate},
		{"unlisted", "unlisted", entity.VideoVisibilityUnlisted},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := new(MockVideoRepo)
			uc := video.New(repo, nil, nil, nil, nil, nil)

			repo.On("Store", mock.Anything, mock.MatchedBy(func(v *entity.Video) bool {
				return v.Visibility == tc.want
			})).Return(nil)

			_, err := uc.CreateUpload(context.Background(), "user-1", request.CreateVideoUpload{
				Title:      "My video",
				FileName:   "clip.mp4",
				Visibility: tc.visibility,
			})

			require.NoError(t, err)
			repo.AssertExpectations(t)
		})
	}
}

func TestCreateUpload_UnsupportedExtension(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	uc := video.New(repo, nil, nil, nil, nil, nil)

	_, err := uc.CreateUpload(context.Background(), "user-1", request.CreateVideoUpload{
		Title:    "Sketchy file",
		FileName: "malware.exe",
	})

	require.ErrorIs(t, err, entity.ErrUnsupportedVideoFormat)
	repo.AssertNotCalled(t, "Store", mock.Anything, mock.Anything)
}

// applyResult makes the mock repo report an applied event on a video that
// was in `before` state just before it.
func applyResult(repo *MockVideoRepo, before entity.Video, applied bool, reason string) {
	repo.On("ApplyTranscodeEvent", mock.Anything, mock.Anything, mock.Anything).
		Return(entity.TranscodeEventResult{Applied: applied, Reason: reason, Before: before}, nil)
}

func TestHandleTranscodeEvent_NotifiesFollowersOnFirstPublicComplete(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	notif := newFakeNotification()
	uc := video.New(repo, nil, nil, nil, nil, notif)

	applyResult(repo, entity.Video{ID: "v1", UserID: "user-1", Status: entity.VideoStatusProcessing, Visibility: entity.VideoVisibilityPublic}, true, "")

	_, err := uc.HandleTranscodeEvent(context.Background(), completedEvent("v1"))

	require.NoError(t, err)
	notif.expectCall(t)
}

func TestHandleTranscodeEvent_NoNotifyForDuplicate(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	notif := newFakeNotification()
	uc := video.New(repo, nil, nil, nil, nil, notif)

	// A redelivered COMPLETED is a duplicate: followers must not be spammed
	// a second time.
	applyResult(repo, entity.Video{}, false, entity.TranscodeSkipDuplicate)

	_, err := uc.HandleTranscodeEvent(context.Background(), completedEvent("v2"))

	require.NoError(t, err)
	notif.expectNoCall(t)
}

func TestHandleTranscodeEvent_NoNotifyWhenPrivate(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	notif := newFakeNotification()
	uc := video.New(repo, nil, nil, nil, nil, notif)

	applyResult(repo, entity.Video{ID: "v3", UserID: "user-1", Status: entity.VideoStatusProcessing, Visibility: entity.VideoVisibilityPrivate}, true, "")

	_, err := uc.HandleTranscodeEvent(context.Background(), completedEvent("v3"))

	require.NoError(t, err)
	notif.expectNoCall(t)
}

func TestHandleTranscodeEvent_NoNotifyWhenFailed(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	notif := newFakeNotification()
	uc := video.New(repo, nil, nil, nil, nil, notif)

	applyResult(repo, entity.Video{ID: "v4", UserID: "user-1", Status: entity.VideoStatusProcessing, Visibility: entity.VideoVisibilityPublic}, true, "")

	_, err := uc.HandleTranscodeEvent(context.Background(), entity.TranscodeEvent{
		IdempotencyKey: "job-1:FAILED", JobID: "job-1", VideoID: "v4", Status: entity.TranscodeEventFailed,
	})

	require.NoError(t, err)
	notif.expectNoCall(t)
}

// fakePublisher records published jobs and optionally fails.
type fakePublisher struct {
	err  error
	jobs []nats.TranscodeJob
}

func (f *fakePublisher) PublishTranscodeJob(_ context.Context, job nats.TranscodeJob) error {
	f.jobs = append(f.jobs, job)
	return f.err
}

func TestConfirmUpload_QueuesJob(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	pub := &fakePublisher{}
	uc := video.New(repo, nil, nil, nil, transcodejob.New(repo, pub, nil), nil)

	pending := entity.Video{ID: "v8", UserID: "owner-1", Status: entity.VideoStatusPending, RawS3Key: "raw-uploads/v8/raw.mp4"}
	queued := pending
	queued.Status = entity.VideoStatusProcessing
	repo.On("GetByID", mock.Anything, "v8").Return(pending, nil).Once()
	repo.On("MarkQueued", mock.Anything, "v8", mock.Anything, entity.VideoStatusPending, mock.Anything).Return(true, nil)
	repo.On("GetByID", mock.Anything, "v8").Return(queued, nil).Once()

	res, err := uc.ConfirmUpload(context.Background(), "owner-1", request.ConfirmUpload{VideoID: "v8"})

	require.NoError(t, err)
	assert.Equal(t, string(entity.VideoStatusProcessing), res.Status)
	require.Len(t, pub.jobs, 1)
	assert.Equal(t, "raw-uploads/v8/raw.mp4", pub.jobs[0].SourceKey)
	assert.Equal(t, []string{"720p", "480p"}, pub.jobs[0].Renditions)
	repo.AssertExpectations(t)
}

// If NATS is down the job must not vanish silently: the video goes back to
// pending and the client gets an error it can retry on.
func TestConfirmUpload_PublishFailureRollsBack(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	pub := &fakePublisher{err: errors.New("nats: no responders")}
	uc := video.New(repo, nil, nil, nil, transcodejob.New(repo, pub, nil), nil)

	repo.On("GetByID", mock.Anything, "v9").Return(entity.Video{ID: "v9", UserID: "owner-1", Status: entity.VideoStatusPending}, nil)
	repo.On("MarkQueued", mock.Anything, "v9", mock.Anything, entity.VideoStatusPending, mock.Anything).Return(true, nil)
	repo.On("RevertQueued", mock.Anything, "v9", mock.Anything, entity.VideoStatusPending, mock.Anything).Return(nil)

	_, err := uc.ConfirmUpload(context.Background(), "owner-1", request.ConfirmUpload{VideoID: "v9"})

	require.ErrorIs(t, err, entity.ErrTranscodeQueueUnavailable)
	repo.AssertExpectations(t)
}

func TestConfirmUpload_AlreadyConfirmedIsNoop(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	pub := &fakePublisher{}
	uc := video.New(repo, nil, nil, nil, transcodejob.New(repo, pub, nil), nil)

	repo.On("GetByID", mock.Anything, "v10").Return(entity.Video{ID: "v10", UserID: "owner-1", Status: entity.VideoStatusProcessing}, nil)

	_, err := uc.ConfirmUpload(context.Background(), "owner-1", request.ConfirmUpload{VideoID: "v10"})

	require.NoError(t, err)
	assert.Empty(t, pub.jobs, "a second confirm must not enqueue another job")
	repo.AssertNotCalled(t, "MarkQueued", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRetryTranscode_RequeuesFailedVideo(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	pub := &fakePublisher{}
	uc := video.New(repo, nil, nil, nil, transcodejob.New(repo, pub, nil), nil)

	existing := entity.Video{
		ID: "v5", UserID: "owner-1", Status: entity.VideoStatusFailed, RawS3Key: "raw-uploads/v5/raw.mp4",
	}
	repo.On("GetByID", mock.Anything, "v5").Return(existing, nil)
	repo.On("MarkQueued", mock.Anything, "v5", mock.Anything, entity.VideoStatusFailed, mock.Anything).Return(true, nil)

	res, err := uc.RetryTranscode(context.Background(), "owner-1", "v5")

	require.NoError(t, err)
	assert.Equal(t, "v5", res.ID)
	require.Len(t, pub.jobs, 1, "a new job is published, reusing the uploaded raw file")
	assert.Equal(t, "raw-uploads/v5/raw.mp4", pub.jobs[0].SourceKey)
	repo.AssertExpectations(t)
}

func TestRetryTranscode_ForbiddenForNonOwner(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	uc := video.New(repo, nil, nil, nil, transcodejob.New(repo, &fakePublisher{}, nil), nil)

	existing := entity.Video{ID: "v6", UserID: "owner-1", Status: entity.VideoStatusFailed}
	repo.On("GetByID", mock.Anything, "v6").Return(existing, nil)

	_, err := uc.RetryTranscode(context.Background(), "someone-else", "v6")

	require.ErrorIs(t, err, entity.ErrVideoForbidden)
	repo.AssertNotCalled(t, "MarkQueued", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRetryTranscode_ErrorsWhenNotFailed(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	uc := video.New(repo, nil, nil, nil, transcodejob.New(repo, &fakePublisher{}, nil), nil)

	existing := entity.Video{ID: "v7", UserID: "owner-1", Status: entity.VideoStatusComplete}
	repo.On("GetByID", mock.Anything, "v7").Return(existing, nil)

	_, err := uc.RetryTranscode(context.Background(), "owner-1", "v7")

	require.ErrorIs(t, err, entity.ErrVideoNotFailed)
	repo.AssertNotCalled(t, "MarkQueued", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestCreateUpload_UploadURLUsesPublicBase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		base string // "" = not configured
		want string
	}{
		{"defaults to the local MinIO", "", "http://localhost:9000/raw-videos/"},
		{"public address from MINIO_PUBLIC_URL", "https://media.example.com", "https://media.example.com/raw-videos/"},
		{"trailing slash is ignored", "http://203.0.113.7:9000/", "http://203.0.113.7:9000/raw-videos/"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := new(MockVideoRepo)
			repo.On("Store", mock.Anything, mock.Anything).Return(nil)
			uc := video.New(repo, nil, nil, nil, nil, nil)
			if tc.base != "" {
				uc.WithUploadBaseURL(tc.base)
			}

			res, err := uc.CreateUpload(context.Background(), "user-1", request.CreateVideoUpload{Title: "t", FileName: "clip.mp4"})

			require.NoError(t, err)
			require.True(t, strings.HasPrefix(res.UploadUrl, tc.want), res.UploadUrl)
			require.Contains(t, res.UploadUrl, res.RawS3Key)
		})
	}
}

type fakeSigner struct {
	bucket, key string
	ttl         time.Duration
	err         error
}

func (f *fakeSigner) PresignPut(_ context.Context, bucket, key string, ttl time.Duration) (string, error) {
	f.bucket, f.key, f.ttl = bucket, key, ttl
	return "https://media.example.com/" + bucket + "/" + key + "?X-Amz-Signature=abc", f.err
}

func TestCreateUpload_PresignedURL(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	repo.On("Store", mock.Anything, mock.Anything).Return(nil)
	signer := &fakeSigner{}
	uc := video.New(repo, nil, nil, nil, nil, nil).WithUploadSigner(signer, "raw-videos")

	res, err := uc.CreateUpload(context.Background(), "user-1", request.CreateVideoUpload{Title: "t", FileName: "clip.mp4"})

	require.NoError(t, err)
	require.Equal(t, "raw-videos", signer.bucket)
	require.Equal(t, res.RawS3Key, signer.key)
	require.Equal(t, 2*time.Hour, signer.ttl)
	require.Contains(t, res.UploadUrl, "X-Amz-Signature=")
}

func TestCreateUpload_SignerFailureFailsTheRequest(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	repo.On("Store", mock.Anything, mock.Anything).Return(nil)
	uc := video.New(repo, nil, nil, nil, nil, nil).WithUploadSigner(&fakeSigner{err: errors.New("boom")}, "raw-videos")

	_, err := uc.CreateUpload(context.Background(), "user-1", request.CreateVideoUpload{Title: "t", FileName: "clip.mp4"})

	require.Error(t, err)
}

func TestCreateImageUpload_PresignedPublicBucket(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	signer := &fakeSigner{}
	uc := video.New(repo, nil, nil, nil, nil, nil).
		WithUploadBaseURL("https://media.example.com/").
		WithUploadSigner(signer, "raw-videos").
		WithPublicBucket("public-assets")

	res, err := uc.CreateImageUpload(context.Background(), "user-1", request.CreateImageUpload{FileName: "Me.PNG"})

	require.NoError(t, err)
	require.Equal(t, "public-assets", signer.bucket)
	require.Equal(t, 15*time.Minute, signer.ttl)
	require.Contains(t, signer.key, "images/user-1/")
	require.True(t, strings.HasSuffix(signer.key, ".png"))
	require.Contains(t, res.UploadUrl, "X-Amz-Signature=")
	require.Equal(t, "https://media.example.com/public-assets/"+signer.key, res.PublicUrl)
	repo.AssertNotCalled(t, "Store", mock.Anything, mock.Anything)
}

func TestCreateImageUpload_UnsupportedExtension(t *testing.T) {
	t.Parallel()

	uc := video.New(new(MockVideoRepo), nil, nil, nil, nil, nil).WithPublicBucket("public-assets")

	_, err := uc.CreateImageUpload(context.Background(), "user-1", request.CreateImageUpload{FileName: "clip.mp4"})

	require.ErrorIs(t, err, entity.ErrUnsupportedImageFormat)
}

func TestCreateImageUpload_NoSignerUsesBareURL(t *testing.T) {
	t.Parallel()

	uc := video.New(new(MockVideoRepo), nil, nil, nil, nil, nil).WithPublicBucket("public-assets")

	res, err := uc.CreateImageUpload(context.Background(), "user-1", request.CreateImageUpload{FileName: "a.jpg"})

	require.NoError(t, err)
	require.Equal(t, res.PublicUrl, res.UploadUrl)
}
