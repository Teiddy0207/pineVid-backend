package video_test

import (
	"context"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/internal/usecase/video"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockVideoRepo is a mock implementation of repo.VideoRepo
type MockVideoRepo struct {
	mock.Mock
}

func (m *MockVideoRepo) Store(ctx context.Context, v *entity.Video) error {
	args := m.Called(ctx, v)
	return args.Error(0)
}

func (m *MockVideoRepo) GetByID(ctx context.Context, id string) (entity.Video, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(entity.Video), args.Error(1)
}

func (m *MockVideoRepo) GetByIDs(ctx context.Context, ids []string) ([]entity.Video, error) {
	return nil, nil
}

func (m *MockVideoRepo) List(ctx context.Context, filter repo.VideoFilter) ([]entity.Video, int, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]entity.Video), args.Int(1), args.Error(2)
}

func (m *MockVideoRepo) Update(ctx context.Context, v *entity.Video) error {
	args := m.Called(ctx, v)
	return args.Error(0)
}

func (m *MockVideoRepo) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestHandleTranscodeCallback_Success(t *testing.T) {
	mockRepo := new(MockVideoRepo)
	uc := video.New(mockRepo, nil, nil, nil, nil, nil)

	videoID := "test-video-123"
	existingVideo := entity.Video{
		ID:        videoID,
		UserID:    "user-456",
		Title:     "Test Video",
		Status:    entity.VideoStatusProcessing,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockRepo.On("GetByID", mock.Anything, videoID).Return(existingVideo, nil)
	mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(v *entity.Video) bool {
		return v.ID == videoID && v.Status == entity.VideoStatusComplete && v.HLSUrl == "/hls-streams/videos/test-video-123/master.m3u8"
	})).Return(nil)

	ownerID, err := uc.HandleTranscodeCallback(context.Background(), videoID, "complete", "/hls-streams/videos/test-video-123/master.m3u8")

	assert.NoError(t, err)
	assert.Equal(t, "user-456", ownerID)
	mockRepo.AssertExpectations(t)
}

func TestHandleTranscodeCallback_Failed(t *testing.T) {
	mockRepo := new(MockVideoRepo)
	uc := video.New(mockRepo, nil, nil, nil, nil, nil)

	videoID := "test-video-456"
	existingVideo := entity.Video{
		ID:        videoID,
		UserID:    "user-789",
		Title:     "Failed Video",
		Status:    entity.VideoStatusProcessing,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockRepo.On("GetByID", mock.Anything, videoID).Return(existingVideo, nil)
	mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(v *entity.Video) bool {
		return v.ID == videoID && v.Status == entity.VideoStatusFailed
	})).Return(nil)

	ownerID, err := uc.HandleTranscodeCallback(context.Background(), videoID, "failed", "")

	assert.NoError(t, err)
	assert.Equal(t, "user-789", ownerID)
	mockRepo.AssertExpectations(t)
}

// NATS redelivers a job when the ack is lost; the second identical
// "complete" callback must not write again (or re-notify followers).
func TestHandleTranscodeCallback_DuplicateCompleteIsNoop(t *testing.T) {
	mockRepo := new(MockVideoRepo)
	uc := video.New(mockRepo, nil, nil, nil, nil, nil)

	videoID := "dup-video"
	mockRepo.On("GetByID", mock.Anything, videoID).Return(entity.Video{
		ID: videoID, UserID: "owner", Status: entity.VideoStatusComplete, HLSUrl: "/hls/master.m3u8",
	}, nil)

	ownerID, err := uc.HandleTranscodeCallback(context.Background(), videoID, "complete", "/hls/master.m3u8")

	assert.NoError(t, err)
	assert.Equal(t, "owner", ownerID)
	mockRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

// A late "failed" from a redelivered attempt must not overwrite a video a
// previous attempt already completed.
func TestHandleTranscodeCallback_LateFailureAfterCompleteIgnored(t *testing.T) {
	mockRepo := new(MockVideoRepo)
	uc := video.New(mockRepo, nil, nil, nil, nil, nil)

	mockRepo.On("GetByID", mock.Anything, "v1").Return(entity.Video{
		ID: "v1", UserID: "owner", Status: entity.VideoStatusComplete, HLSUrl: "/hls/master.m3u8",
	}, nil)

	_, err := uc.HandleTranscodeCallback(context.Background(), "v1", "failed", "")

	assert.NoError(t, err)
	mockRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

// An admin takedown can't be undone by a (possibly forged) callback.
func TestHandleTranscodeCallback_BannedVideoStaysBanned(t *testing.T) {
	mockRepo := new(MockVideoRepo)
	uc := video.New(mockRepo, nil, nil, nil, nil, nil)

	mockRepo.On("GetByID", mock.Anything, "v2").Return(entity.Video{
		ID: "v2", UserID: "owner", Status: entity.VideoStatusBanned,
	}, nil)

	_, err := uc.HandleTranscodeCallback(context.Background(), "v2", "complete", "/evil/master.m3u8")

	assert.NoError(t, err)
	mockRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestGetByID_Visibility(t *testing.T) {
	cases := []struct {
		name    string
		video   entity.Video
		viewer  string
		wantErr error
	}{
		{"public complete, anonymous", entity.Video{UserID: "owner", Status: entity.VideoStatusComplete, Visibility: entity.VideoVisibilityPublic}, "", nil},
		{"unlisted, anonymous with link", entity.Video{UserID: "owner", Status: entity.VideoStatusComplete, Visibility: entity.VideoVisibilityUnlisted}, "", nil},
		{"private, stranger", entity.Video{UserID: "owner", Status: entity.VideoStatusComplete, Visibility: entity.VideoVisibilityPrivate}, "stranger", entity.ErrVideoNotFound},
		{"private, anonymous", entity.Video{UserID: "owner", Status: entity.VideoStatusComplete, Visibility: entity.VideoVisibilityPrivate}, "", entity.ErrVideoNotFound},
		{"private, owner", entity.Video{UserID: "owner", Status: entity.VideoStatusComplete, Visibility: entity.VideoVisibilityPrivate}, "owner", nil},
		{"banned, stranger", entity.Video{UserID: "owner", Status: entity.VideoStatusBanned, Visibility: entity.VideoVisibilityPublic}, "stranger", entity.ErrVideoNotFound},
		{"banned, owner", entity.Video{UserID: "owner", Status: entity.VideoStatusBanned, Visibility: entity.VideoVisibilityPublic}, "owner", nil},
		{"still processing, stranger", entity.Video{UserID: "owner", Status: entity.VideoStatusProcessing, Visibility: entity.VideoVisibilityPublic}, "stranger", entity.ErrVideoNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := new(MockVideoRepo)
			uc := video.New(mockRepo, nil, nil, nil, nil, nil)
			tc.video.ID = "vid"
			mockRepo.On("GetByID", mock.Anything, "vid").Return(tc.video, nil)

			_, err := uc.GetByID(context.Background(), "vid", tc.viewer)
			assert.ErrorIs(t, err, tc.wantErr)

			err = uc.EnsureViewable(context.Background(), "vid", tc.viewer)
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}
