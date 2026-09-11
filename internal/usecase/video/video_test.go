package video_test

import (
	"context"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase/video"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fakeNotification implements usecase.Notification, recording NotifyFollowers
// calls onto a buffered channel so async goroutine-fired notifications (see
// HandleTranscodeCallback) can be observed deterministically in tests instead
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

func TestHandleTranscodeCallback_NotifiesFollowersOnFirstPublicComplete(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	notif := newFakeNotification()
	uc := video.New(repo, nil, nil, nil, nil, notif)

	existing := entity.Video{
		ID: "v1", UserID: "user-1", Status: entity.VideoStatusProcessing,
		Visibility: entity.VideoVisibilityPublic,
	}
	repo.On("GetByID", mock.Anything, "v1").Return(existing, nil)
	repo.On("Update", mock.Anything, mock.Anything).Return(nil)

	err := uc.HandleTranscodeCallback(context.Background(), "v1", "complete", "/hls/v1/master.m3u8")

	require.NoError(t, err)
	notif.expectCall(t)
}

func TestHandleTranscodeCallback_NoNotifyWhenAlreadyComplete(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	notif := newFakeNotification()
	uc := video.New(repo, nil, nil, nil, nil, notif)

	existing := entity.Video{
		ID: "v2", UserID: "user-1", Status: entity.VideoStatusComplete,
		Visibility: entity.VideoVisibilityPublic,
	}
	repo.On("GetByID", mock.Anything, "v2").Return(existing, nil)
	repo.On("Update", mock.Anything, mock.Anything).Return(nil)

	// A duplicate/retried transcode callback for an already-complete video
	// must not spam followers a second time.
	err := uc.HandleTranscodeCallback(context.Background(), "v2", "complete", "/hls/v2/master.m3u8")

	require.NoError(t, err)
	notif.expectNoCall(t)
}

func TestHandleTranscodeCallback_NoNotifyWhenPrivate(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	notif := newFakeNotification()
	uc := video.New(repo, nil, nil, nil, nil, notif)

	existing := entity.Video{
		ID: "v3", UserID: "user-1", Status: entity.VideoStatusProcessing,
		Visibility: entity.VideoVisibilityPrivate,
	}
	repo.On("GetByID", mock.Anything, "v3").Return(existing, nil)
	repo.On("Update", mock.Anything, mock.Anything).Return(nil)

	err := uc.HandleTranscodeCallback(context.Background(), "v3", "complete", "/hls/v3/master.m3u8")

	require.NoError(t, err)
	notif.expectNoCall(t)
}

func TestHandleTranscodeCallback_NoNotifyWhenFailed(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	notif := newFakeNotification()
	uc := video.New(repo, nil, nil, nil, nil, notif)

	existing := entity.Video{
		ID: "v4", UserID: "user-1", Status: entity.VideoStatusProcessing,
		Visibility: entity.VideoVisibilityPublic,
	}
	repo.On("GetByID", mock.Anything, "v4").Return(existing, nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(v *entity.Video) bool {
		return v.Status == entity.VideoStatusFailed
	})).Return(nil)

	err := uc.HandleTranscodeCallback(context.Background(), "v4", "failed", "")

	require.NoError(t, err)
	notif.expectNoCall(t)
	assert.Equal(t, "v4", existing.ID) // sanity: existing struct untouched by pointer aliasing surprises
}

func TestRetryTranscode_RequeuesFailedVideo(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	uc := video.New(repo, nil, nil, nil, nil, nil)

	existing := entity.Video{
		ID: "v5", UserID: "owner-1", Status: entity.VideoStatusFailed, RawS3Key: "raw-uploads/v5/raw.mp4",
	}
	repo.On("GetByID", mock.Anything, "v5").Return(existing, nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(v *entity.Video) bool {
		return v.Status == entity.VideoStatusProcessing
	})).Return(nil)

	res, err := uc.RetryTranscode(context.Background(), "owner-1", "v5")

	require.NoError(t, err)
	assert.Equal(t, "v5", res.ID)
	repo.AssertExpectations(t)
}

func TestRetryTranscode_ForbiddenForNonOwner(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	uc := video.New(repo, nil, nil, nil, nil, nil)

	existing := entity.Video{ID: "v6", UserID: "owner-1", Status: entity.VideoStatusFailed}
	repo.On("GetByID", mock.Anything, "v6").Return(existing, nil)

	_, err := uc.RetryTranscode(context.Background(), "someone-else", "v6")

	require.ErrorIs(t, err, entity.ErrVideoForbidden)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestRetryTranscode_ErrorsWhenNotFailed(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	uc := video.New(repo, nil, nil, nil, nil, nil)

	existing := entity.Video{ID: "v7", UserID: "owner-1", Status: entity.VideoStatusComplete}
	repo.On("GetByID", mock.Anything, "v7").Return(existing, nil)

	_, err := uc.RetryTranscode(context.Background(), "owner-1", "v7")

	require.ErrorIs(t, err, entity.ErrVideoNotFailed)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}
