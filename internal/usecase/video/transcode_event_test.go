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
	"github.com/stretchr/testify/require"
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

func (m *MockVideoRepo) SetStatus(ctx context.Context, id string, status entity.VideoStatus, at time.Time) error {
	return m.Called(ctx, id, status, at).Error(0)
}

func (m *MockVideoRepo) MarkQueued(ctx context.Context, id, jobID string, from entity.VideoStatus, at time.Time) (bool, error) {
	args := m.Called(ctx, id, jobID, from, at)
	return args.Bool(0), args.Error(1)
}

func (m *MockVideoRepo) RevertQueued(ctx context.Context, id, jobID string, to entity.VideoStatus, at time.Time) error {
	return m.Called(ctx, id, jobID, to, at).Error(0)
}

func (m *MockVideoRepo) ApplyTranscodeEvent(ctx context.Context, ev entity.TranscodeEvent, at time.Time) (entity.TranscodeEventResult, error) {
	args := m.Called(ctx, ev, at)
	return args.Get(0).(entity.TranscodeEventResult), args.Error(1)
}

func (m *MockVideoRepo) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func completedEvent(videoID string) entity.TranscodeEvent {
	return entity.TranscodeEvent{
		IdempotencyKey: "job-1:COMPLETED",
		JobID:          "job-1",
		VideoID:        videoID,
		Status:         entity.TranscodeEventCompleted,
		HLSMasterURL:   "/hls-streams/videos/" + videoID + "/master.m3u8",
	}
}

func TestHandleTranscodeEvent_ForwardsRepoResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		res  entity.TranscodeEventResult
	}{
		{"applied", entity.TranscodeEventResult{Applied: true, Before: entity.Video{ID: "v1", UserID: "owner"}}},
		{"duplicate", entity.TranscodeEventResult{Reason: entity.TranscodeSkipDuplicate}},
		{"banned video stays banned", entity.TranscodeEventResult{Reason: entity.TranscodeSkipInvalidTransition, Before: entity.Video{Status: entity.VideoStatusBanned}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mockRepo := new(MockVideoRepo)
			uc := video.New(mockRepo, nil, nil, nil, nil, nil)

			ev := completedEvent("v1")
			mockRepo.On("ApplyTranscodeEvent", mock.Anything, ev, mock.Anything).Return(tc.res, nil)

			got, err := uc.HandleTranscodeEvent(context.Background(), ev)
			require.NoError(t, err)
			assert.Equal(t, tc.res, got)
			mockRepo.AssertExpectations(t)
		})
	}
}

// A key that doesn't match the body is rejected before touching the DB, so a
// forged key can't burn the real event's idempotency slot.
func TestHandleTranscodeEvent_RejectsMismatchedKey(t *testing.T) {
	t.Parallel()
	mockRepo := new(MockVideoRepo)
	uc := video.New(mockRepo, nil, nil, nil, nil, nil)

	ev := completedEvent("v1")
	ev.IdempotencyKey = "job-1:PROCESSING"

	_, err := uc.HandleTranscodeEvent(context.Background(), ev)
	require.ErrorIs(t, err, entity.ErrInvalidIdempotencyKey)
	mockRepo.AssertNotCalled(t, "ApplyTranscodeEvent", mock.Anything, mock.Anything, mock.Anything)
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
