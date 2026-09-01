package livestream_test

import (
	"context"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase/livestream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockLivestreamRepo is a hand-rolled testify mock implementing
// repo.LivestreamRepo, following the same pattern as
// internal/usecase/video/transcode_callback_test.go's MockVideoRepo — each
// usecase subpackage's tests mock the shared repo interfaces locally since
// generated *_test.go mocks aren't importable across packages.
type MockLivestreamRepo struct {
	mock.Mock
}

func (m *MockLivestreamRepo) Store(ctx context.Context, ls *entity.Livestream) error {
	args := m.Called(ctx, ls)
	return args.Error(0)
}

func (m *MockLivestreamRepo) GetByID(ctx context.Context, id string) (entity.Livestream, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(entity.Livestream), args.Error(1)
}

func (m *MockLivestreamRepo) GetByUserID(ctx context.Context, userID string) (entity.Livestream, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(entity.Livestream), args.Error(1)
}

func (m *MockLivestreamRepo) GetByStreamKey(ctx context.Context, streamKey string) (entity.Livestream, error) {
	args := m.Called(ctx, streamKey)
	return args.Get(0).(entity.Livestream), args.Error(1)
}

func (m *MockLivestreamRepo) ListActive(ctx context.Context, category string, limit, offset int) ([]entity.Livestream, int, error) {
	args := m.Called(ctx, category, limit, offset)
	return args.Get(0).([]entity.Livestream), args.Int(1), args.Error(2)
}

func (m *MockLivestreamRepo) Update(ctx context.Context, ls *entity.Livestream) error {
	args := m.Called(ctx, ls)
	return args.Error(0)
}

func (m *MockLivestreamRepo) CountActive(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockLivestreamRepo) SumActiveViewers(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

const testGrace = 50 * time.Millisecond

func newLivestreamUseCase(repo *MockLivestreamRepo) *livestream.UseCase {
	return livestream.New(repo, nil, nil, nil, nil, nil, "", "", testGrace)
}

func TestAuthenticateStreamKey_InvalidKey(t *testing.T) {
	t.Parallel()

	repo := new(MockLivestreamRepo)
	uc := newLivestreamUseCase(repo)

	repo.On("GetByStreamKey", mock.Anything, "bad-key").Return(entity.Livestream{}, entity.ErrLivestreamNotFound)

	ok, err := uc.AuthenticateStreamKey(context.Background(), request.StreamKeyAuth{StreamKey: "bad-key"})

	assert.False(t, ok)
	assert.ErrorIs(t, err, entity.ErrInvalidStreamKey)
}

func TestUnpublishStream_ReconnectWithinGrace_StaysLive(t *testing.T) {
	t.Parallel()

	repo := new(MockLivestreamRepo)
	uc := newLivestreamUseCase(repo)

	ls := entity.Livestream{ID: "ls-1", StreamKey: "sk-1", UserID: "user-1", IsLive: true}
	repo.On("GetByStreamKey", mock.Anything, "sk-1").Return(ls, nil).Once()

	err := uc.UnpublishStream(context.Background(), "sk-1")
	assert.NoError(t, err)

	// Reconnect before the grace period elapses.
	ok, err := uc.AuthenticateStreamKey(context.Background(), request.StreamKeyAuth{StreamKey: "sk-1"})
	assert.NoError(t, err)
	assert.True(t, ok)

	// Wait past the original grace window to make sure the pending finalize
	// was actually cancelled, not just delayed.
	time.Sleep(testGrace * 3)

	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestUnpublishStream_NoReconnect_FinalizesAsOffline(t *testing.T) {
	t.Parallel()

	repo := new(MockLivestreamRepo)
	uc := newLivestreamUseCase(repo)

	ls := entity.Livestream{ID: "ls-2", StreamKey: "sk-2", UserID: "user-2", IsLive: true}
	repo.On("GetByStreamKey", mock.Anything, "sk-2").Return(ls, nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(l *entity.Livestream) bool {
		return l.ID == "ls-2" && !l.IsLive && l.EndedAt != nil
	})).Return(nil)

	err := uc.UnpublishStream(context.Background(), "sk-2")
	assert.NoError(t, err)

	time.Sleep(testGrace * 3)

	repo.AssertCalled(t, "Update", mock.Anything, mock.MatchedBy(func(l *entity.Livestream) bool {
		return l.ID == "ls-2" && !l.IsLive
	}))
}

func TestHandleDVRComplete_DuringGrace_DiscardedOnReconnect(t *testing.T) {
	t.Parallel()

	repo := new(MockLivestreamRepo)
	uc := newLivestreamUseCase(repo)

	ls := entity.Livestream{ID: "ls-3", StreamKey: "sk-3", UserID: "user-3", IsLive: true}
	repo.On("GetByStreamKey", mock.Anything, "sk-3").Return(ls, nil).Once()

	err := uc.UnpublishStream(context.Background(), "sk-3")
	assert.NoError(t, err)

	// SRS's on_dvr fires for the interrupted segment while we're still
	// waiting — this must not immediately create a replay video.
	err = uc.HandleDVRComplete(context.Background(), "sk-3")
	assert.NoError(t, err)

	// Streamer reconnects before the grace period elapses: the buffered DVR
	// segment must be discarded, not processed.
	ok, err := uc.AuthenticateStreamKey(context.Background(), request.StreamKeyAuth{StreamKey: "sk-3"})
	assert.NoError(t, err)
	assert.True(t, ok)

	time.Sleep(testGrace * 3)

	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestHandleDVRComplete_NoPending_ProcessesImmediately(t *testing.T) {
	t.Parallel()

	repo := new(MockLivestreamRepo)
	uc := newLivestreamUseCase(repo)

	ls := entity.Livestream{ID: "ls-4", StreamKey: "sk-4", UserID: "user-4"}
	repo.On("GetByStreamKey", mock.Anything, "sk-4").Return(ls, nil)

	// No prior UnpublishStream call, so there's no pending grace window —
	// on_dvr must be handled right away. minioClient/videoRepo are nil in
	// this test double, so the immediate processDVR attempt surfaces as this
	// specific configuration error rather than silently succeeding, which is
	// enough to prove the immediate (non-buffered) code path ran.
	err := uc.HandleDVRComplete(context.Background(), "sk-4")

	assert.ErrorContains(t, err, "DVR pipeline not configured")
}
