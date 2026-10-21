package livestream_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/events"
	"github.com/evrone/go-clean-template/internal/usecase/livestream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
	return livestream.New(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, "", "", testGrace)
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

func TestAuthenticateStreamKey_BannedStreamRejected(t *testing.T) {
	t.Parallel()

	repo := new(MockLivestreamRepo)
	uc := newLivestreamUseCase(repo)

	repo.On("GetByStreamKey", mock.Anything, "sk-banned").Return(entity.Livestream{ID: "ls-b", StreamKey: "sk-banned", IsBanned: true}, nil)

	ok, err := uc.AuthenticateStreamKey(context.Background(), request.StreamKeyAuth{StreamKey: "sk-banned"})
	assert.False(t, ok)
	assert.ErrorIs(t, err, entity.ErrStreamBanned)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestAuthenticateStreamKey_BannedOwnerRejected(t *testing.T) {
	t.Parallel()

	repo := new(MockLivestreamRepo)
	uc := newLivestreamUseCase(repo)

	repo.On("GetByStreamKey", mock.Anything, "sk-owner-banned").Return(entity.Livestream{ID: "ls-o", StreamKey: "sk-owner-banned", OwnerBanned: true}, nil)

	ok, err := uc.AuthenticateStreamKey(context.Background(), request.StreamKeyAuth{StreamKey: "sk-owner-banned"})
	assert.False(t, ok)
	assert.ErrorIs(t, err, entity.ErrStreamBanned)
}

// A ban issued while the streamer is in the reconnect grace window must also
// block the reconnect (the grace shortcut used to skip every DB check).
func TestAuthenticateStreamKey_BanDuringGraceBlocksReconnect(t *testing.T) {
	t.Parallel()

	repo := new(MockLivestreamRepo)
	uc := newLivestreamUseCase(repo)

	live := entity.Livestream{ID: "ls-g", StreamKey: "sk-g", UserID: "user-g", IsLive: true}
	repo.On("GetByStreamKey", mock.Anything, "sk-g").Return(live, nil).Once()
	require.NoError(t, uc.UnpublishStream(context.Background(), "sk-g"))

	banned := live
	banned.IsBanned = true
	repo.On("GetByStreamKey", mock.Anything, "sk-g").Return(banned, nil)
	repo.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()

	ok, err := uc.AuthenticateStreamKey(context.Background(), request.StreamKeyAuth{StreamKey: "sk-g"})
	assert.False(t, ok)
	assert.ErrorIs(t, err, entity.ErrStreamBanned)
}

func TestSendChatMessage_Validation(t *testing.T) {
	t.Parallel()

	uc := newLivestreamUseCase(new(MockLivestreamRepo))

	_, err := uc.SendChatMessage(context.Background(), "room", "u-alice", "alice", "", "   ")
	assert.ErrorIs(t, err, entity.ErrEmptyChatMessage)

	_, err = uc.SendChatMessage(context.Background(), "room", "u-alice", "alice", "", strings.Repeat("x", 501))
	assert.ErrorIs(t, err, entity.ErrChatMessageTooLong)

	msg, err := uc.SendChatMessage(context.Background(), "room", "u-alice", "alice", "", "  hi  ")
	require.NoError(t, err)
	assert.Equal(t, "hi", msg.Text)
	assert.Equal(t, "alice", msg.User.Name)
	assert.Equal(t, "A", msg.User.Avatar)
}

func TestUnpublishStream_ReconnectWithinGrace_StaysLive(t *testing.T) {
	t.Parallel()

	repo := new(MockLivestreamRepo)
	uc := newLivestreamUseCase(repo)

	ls := entity.Livestream{ID: "ls-1", StreamKey: "sk-1", UserID: "user-1", IsLive: true}
	// Looked up by UnpublishStream and again by the reconnect's ban check.
	repo.On("GetByStreamKey", mock.Anything, "sk-1").Return(ls, nil).Twice()

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
	repo.On("GetByStreamKey", mock.Anything, "sk-3").Return(ls, nil).Twice()

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

func TestUpdateStreamInfo_SavesThumbnailAndKeepsItWhenOmitted(t *testing.T) {
	t.Parallel()

	repo := new(MockLivestreamRepo)
	uc := newLivestreamUseCase(repo)

	ls := entity.Livestream{ID: "ls-5", UserID: "user-5", Title: "old", Category: "Gaming"}
	repo.On("GetByUserID", mock.Anything, "user-5").Return(ls, nil)
	repo.On("Update", mock.Anything, mock.Anything).Return(nil)

	res, err := uc.UpdateStreamInfo(context.Background(), "user-5", request.UpdateLivestreamInfo{
		Title:        "new title",
		ThumbnailURL: "https://media.example.com/public-assets/images/cover.jpg",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://media.example.com/public-assets/images/cover.jpg", res.ThumbnailURL)
	assert.Equal(t, "Gaming", res.Category)

	saved := repo.Calls[1].Arguments.Get(1).(*entity.Livestream)
	assert.Equal(t, "https://media.example.com/public-assets/images/cover.jpg", saved.ThumbnailURL)

	// Sending only a title must not wipe an existing thumbnail.
	withThumb := entity.Livestream{ID: "ls-6", UserID: "user-6", ThumbnailURL: "https://media.example.com/c.jpg"}
	repo.On("GetByUserID", mock.Anything, "user-6").Return(withThumb, nil)
	res, err = uc.UpdateStreamInfo(context.Background(), "user-6", request.UpdateLivestreamInfo{Title: "only title"})
	require.NoError(t, err)
	assert.Equal(t, "https://media.example.com/c.jpg", res.ThumbnailURL)
}

// fakeLiveEvents records appended events; appendErr makes every append fail.
type fakeLiveEvents struct {
	mu        sync.Mutex
	chats     []entity.LiveEvent
	appendErr error
}

func (f *fakeLiveEvents) AppendChat(_ context.Context, ev entity.LiveEvent) error {
	if f.appendErr != nil {
		return f.appendErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chats = append(f.chats, ev)
	return nil
}

func (f *fakeLiveEvents) AppendHeart(context.Context, entity.LiveEvent) (int64, error) { return 0, nil }
func (f *fakeLiveEvents) HeartTotal(context.Context, string) (int64, error)            { return 7, nil }
func (f *fakeLiveEvents) ResetHearts(context.Context, string) error                    { return nil }

func TestSendChatMessage_AppendsToLiveEventsThenBroadcasts(t *testing.T) {
	t.Parallel()

	hub := events.NewChatHub()
	log := &fakeLiveEvents{}
	uc := livestream.New(new(MockLivestreamRepo), nil, nil, nil, nil, hub, nil, nil, nil, nil, "", "", testGrace).WithLiveEvents(log)

	ch, unsub := hub.Subscribe("room")
	defer unsub()

	_, err := uc.SendChatMessage(context.Background(), "room", "u-alice", "alice", "", "hello")
	require.NoError(t, err)

	require.Len(t, log.chats, 1)
	ev := log.chats[0]
	assert.Equal(t, entity.LiveEventChat, ev.Type)
	assert.Equal(t, "room", ev.LivestreamID)
	assert.Equal(t, "u-alice", ev.UserID)
	assert.Equal(t, "hello", ev.Text)
	assert.False(t, ev.At.IsZero())

	select {
	case msg := <-ch:
		assert.Equal(t, "hello", msg.Text)
	case <-time.After(time.Second):
		t.Fatal("chat line was not broadcast")
	}
}

func TestSendChatMessage_LogDown_FailsWithoutBroadcast(t *testing.T) {
	t.Parallel()

	hub := events.NewChatHub()
	log := &fakeLiveEvents{appendErr: entity.ErrLiveEventsUnavailable}
	uc := livestream.New(new(MockLivestreamRepo), nil, nil, nil, nil, hub, nil, nil, nil, nil, "", "", testGrace).WithLiveEvents(log)

	ch, unsub := hub.Subscribe("room")
	defer unsub()

	_, err := uc.SendChatMessage(context.Background(), "room", "u-alice", "alice", "", "hello")
	assert.ErrorIs(t, err, entity.ErrLiveEventsUnavailable)

	select {
	case msg := <-ch:
		t.Fatalf("an unrecorded message was broadcast: %+v", msg)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestSubscribeChat_SendsCurrentHeartTotalFirst(t *testing.T) {
	t.Parallel()

	hub := events.NewChatHub()
	uc := livestream.New(new(MockLivestreamRepo), nil, nil, nil, nil, hub, nil, nil, nil, nil, "", "", testGrace).WithLiveEvents(&fakeLiveEvents{})

	ch, unsub, err := uc.SubscribeChat("room")
	require.NoError(t, err)
	defer unsub()

	select {
	case msg := <-ch:
		assert.Equal(t, "heart_total", msg.Type)
		assert.EqualValues(t, 7, msg.Value)
	case <-time.After(time.Second):
		t.Fatal("no initial heart total")
	}
}
