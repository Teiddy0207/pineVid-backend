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
	uc := video.New(mockRepo, nil, nil)

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

	err := uc.HandleTranscodeCallback(context.Background(), videoID, "complete", "/hls-streams/videos/test-video-123/master.m3u8")

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestHandleTranscodeCallback_Failed(t *testing.T) {
	mockRepo := new(MockVideoRepo)
	uc := video.New(mockRepo, nil, nil)

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

	err := uc.HandleTranscodeCallback(context.Background(), videoID, "failed", "")

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}
