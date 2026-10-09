package admin_test

import (
	"context"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/internal/usecase/admin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockLivestreamRepo struct{ mock.Mock }

func (m *mockLivestreamRepo) Store(ctx context.Context, ls *entity.Livestream) error {
	return m.Called(ctx, ls).Error(0)
}
func (m *mockLivestreamRepo) GetByID(ctx context.Context, id string) (entity.Livestream, error) {
	a := m.Called(ctx, id)
	return a.Get(0).(entity.Livestream), a.Error(1)
}
func (m *mockLivestreamRepo) GetByUserID(ctx context.Context, userID string) (entity.Livestream, error) {
	a := m.Called(ctx, userID)
	return a.Get(0).(entity.Livestream), a.Error(1)
}
func (m *mockLivestreamRepo) GetByStreamKey(ctx context.Context, key string) (entity.Livestream, error) {
	a := m.Called(ctx, key)
	return a.Get(0).(entity.Livestream), a.Error(1)
}
func (m *mockLivestreamRepo) ListActive(ctx context.Context, category string, limit, offset int) ([]entity.Livestream, int, error) {
	a := m.Called(ctx, category, limit, offset)
	return a.Get(0).([]entity.Livestream), a.Int(1), a.Error(2)
}
func (m *mockLivestreamRepo) Update(ctx context.Context, ls *entity.Livestream) error {
	return m.Called(ctx, ls).Error(0)
}
func (m *mockLivestreamRepo) CountActive(ctx context.Context) (int64, error) {
	a := m.Called(ctx)
	return a.Get(0).(int64), a.Error(1)
}
func (m *mockLivestreamRepo) SumActiveViewers(ctx context.Context) (int64, error) {
	a := m.Called(ctx)
	return a.Get(0).(int64), a.Error(1)
}

type mockVideoRepo struct{ mock.Mock }

func (m *mockVideoRepo) Store(ctx context.Context, v *entity.Video) error {
	return m.Called(ctx, v).Error(0)
}
func (m *mockVideoRepo) GetByID(ctx context.Context, id string) (entity.Video, error) {
	a := m.Called(ctx, id)
	return a.Get(0).(entity.Video), a.Error(1)
}
func (m *mockVideoRepo) GetByIDs(ctx context.Context, ids []string) ([]entity.Video, error) {
	return nil, nil
}
func (m *mockVideoRepo) List(ctx context.Context, filter repo.VideoFilter) ([]entity.Video, int, error) {
	a := m.Called(ctx, filter)
	return a.Get(0).([]entity.Video), a.Int(1), a.Error(2)
}
func (m *mockVideoRepo) Update(ctx context.Context, v *entity.Video) error {
	return m.Called(ctx, v).Error(0)
}
func (m *mockVideoRepo) SetStatus(ctx context.Context, id string, status entity.VideoStatus, at time.Time) error {
	return m.Called(ctx, id, status, at).Error(0)
}
func (m *mockVideoRepo) MarkQueued(ctx context.Context, id, jobID string, from entity.VideoStatus, at time.Time) (bool, error) {
	a := m.Called(ctx, id, jobID, from, at)
	return a.Bool(0), a.Error(1)
}
func (m *mockVideoRepo) RevertQueued(ctx context.Context, id, jobID string, to entity.VideoStatus, at time.Time) error {
	return m.Called(ctx, id, jobID, to, at).Error(0)
}
func (m *mockVideoRepo) ApplyTranscodeEvent(ctx context.Context, ev entity.TranscodeEvent, at time.Time) (entity.TranscodeEventResult, error) {
	a := m.Called(ctx, ev, at)
	return a.Get(0).(entity.TranscodeEventResult), a.Error(1)
}
func (m *mockVideoRepo) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

type mockUserRepo struct{ mock.Mock }

func (m *mockUserRepo) Store(ctx context.Context, u *entity.User) error {
	return m.Called(ctx, u).Error(0)
}
func (m *mockUserRepo) GetByID(ctx context.Context, id string) (entity.User, error) {
	a := m.Called(ctx, id)
	return a.Get(0).(entity.User), a.Error(1)
}
func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (entity.User, error) {
	a := m.Called(ctx, email)
	return a.Get(0).(entity.User), a.Error(1)
}
func (m *mockUserRepo) GetByUsername(ctx context.Context, username string) (entity.User, error) {
	a := m.Called(ctx, username)
	return a.Get(0).(entity.User), a.Error(1)
}
func (m *mockUserRepo) Update(ctx context.Context, u *entity.User) error {
	return m.Called(ctx, u).Error(0)
}
func (m *mockUserRepo) List(ctx context.Context, page, limit int) ([]entity.User, int, error) {
	a := m.Called(ctx, page, limit)
	return a.Get(0).([]entity.User), a.Int(1), a.Error(2)
}

type mockWorkerRepo struct{ mock.Mock }

func (m *mockWorkerRepo) UpsertHeartbeat(ctx context.Context, hb entity.WorkerHeartbeat) error {
	return m.Called(ctx, hb).Error(0)
}
func (m *mockWorkerRepo) ListActive(ctx context.Context, staleAfter time.Duration) ([]entity.WorkerHeartbeat, error) {
	a := m.Called(ctx, staleAfter)
	return a.Get(0).([]entity.WorkerHeartbeat), a.Error(1)
}

type mockAdminStatsRepo struct{ mock.Mock }

func (m *mockAdminStatsRepo) ChannelStats(ctx context.Context) (entity.ChannelStats, error) {
	a := m.Called(ctx)
	return a.Get(0).(entity.ChannelStats), a.Error(1)
}
func (m *mockAdminStatsRepo) CategoryBreakdown(ctx context.Context) ([]entity.CategoryStat, error) {
	a := m.Called(ctx)
	return a.Get(0).([]entity.CategoryStat), a.Error(1)
}

func TestGetDashboard_IncludesChannelAndCategoryStats(t *testing.T) {
	t.Parallel()

	lsRepo := new(mockLivestreamRepo)
	vRepo := new(mockVideoRepo)
	uRepo := new(mockUserRepo)
	wRepo := new(mockWorkerRepo)
	statsRepo := new(mockAdminStatsRepo)

	vRepo.On("List", mock.Anything, mock.Anything).Return([]entity.Video{}, 42, nil)
	lsRepo.On("CountActive", mock.Anything).Return(int64(3), nil)
	lsRepo.On("SumActiveViewers", mock.Anything).Return(int64(999), nil)
	wRepo.On("ListActive", mock.Anything, mock.Anything).Return([]entity.WorkerHeartbeat{{}}, nil)
	statsRepo.On("ChannelStats", mock.Anything).Return(entity.ChannelStats{
		TotalChannels: 30, ActiveChannels: 3, BannedChannels: 2,
	}, nil)
	statsRepo.On("CategoryBreakdown", mock.Anything).Return([]entity.CategoryStat{
		{Category: "Gaming", VideoCount: 15},
		{Category: "Music", VideoCount: 9},
	}, nil)

	uc := admin.New(lsRepo, vRepo, uRepo, wRepo, statsRepo, nil, nil)

	dash, err := uc.GetDashboard(context.Background())

	require.NoError(t, err)
	require.Equal(t, int64(42), dash.TotalVideos)
	require.Equal(t, int64(3), dash.ActiveLivestreams)
	require.Equal(t, int64(1), dash.ActiveWorkers)
	require.Equal(t, int64(999), dash.TotalViewers)
	require.Equal(t, int64(30), dash.TotalChannels)
	require.Equal(t, int64(3), dash.ActiveChannels)
	require.Equal(t, int64(2), dash.BannedChannels)
	require.Len(t, dash.CategoryBreakdown, 2)
	require.Equal(t, "Gaming", dash.CategoryBreakdown[0].Category)
	require.Equal(t, int64(15), dash.CategoryBreakdown[0].VideoCount)
}

type fakeKicker struct{ kicked []string }

func (f *fakeKicker) KickStream(_ context.Context, streamKey string) error {
	f.kicked = append(f.kicked, streamKey)
	return nil
}

type fakeSessions struct {
	invalidated []string
	revoked     []string
}

func (f *fakeSessions) InvalidateBanCache(userID string) { f.invalidated = append(f.invalidated, userID) }
func (f *fakeSessions) RevokeSessions(_ context.Context, userID string) error {
	f.revoked = append(f.revoked, userID)
	return nil
}

func TestBanUser_TakesEffectImmediately(t *testing.T) {
	lsRepo, uRepo := new(mockLivestreamRepo), new(mockUserRepo)
	kicker, sessions := &fakeKicker{}, &fakeSessions{}
	uc := admin.New(lsRepo, new(mockVideoRepo), uRepo, new(mockWorkerRepo), new(mockAdminStatsRepo), kicker, sessions)

	uRepo.On("GetByID", mock.Anything, "u1").Return(entity.User{ID: "u1"}, nil)
	uRepo.On("Update", mock.Anything, mock.MatchedBy(func(u *entity.User) bool { return u.IsBanned })).Return(nil)
	lsRepo.On("GetByUserID", mock.Anything, "u1").Return(entity.Livestream{ID: "ls1", UserID: "u1", StreamKey: "sk1", IsLive: true}, nil)
	lsRepo.On("Update", mock.Anything, mock.MatchedBy(func(ls *entity.Livestream) bool { return !ls.IsLive })).Return(nil)

	require.NoError(t, uc.BanUser(context.Background(), "u1"))

	assert.Equal(t, []string{"u1"}, sessions.invalidated)
	assert.Equal(t, []string{"u1"}, sessions.revoked)
	assert.Equal(t, []string{"sk1"}, kicker.kicked)
}

func TestUnbanUser_OnlyInvalidatesCache(t *testing.T) {
	uRepo := new(mockUserRepo)
	kicker, sessions := &fakeKicker{}, &fakeSessions{}
	uc := admin.New(new(mockLivestreamRepo), new(mockVideoRepo), uRepo, new(mockWorkerRepo), new(mockAdminStatsRepo), kicker, sessions)

	uRepo.On("GetByID", mock.Anything, "u1").Return(entity.User{ID: "u1", IsBanned: true}, nil)
	uRepo.On("Update", mock.Anything, mock.MatchedBy(func(u *entity.User) bool { return !u.IsBanned })).Return(nil)

	require.NoError(t, uc.UnbanUser(context.Background(), "u1"))

	assert.Equal(t, []string{"u1"}, sessions.invalidated)
	assert.Empty(t, sessions.revoked)
	assert.Empty(t, kicker.kicked)
}

func TestBanStream_MarksBannedAndKicks(t *testing.T) {
	lsRepo := new(mockLivestreamRepo)
	kicker := &fakeKicker{}
	uc := admin.New(lsRepo, new(mockVideoRepo), new(mockUserRepo), new(mockWorkerRepo), new(mockAdminStatsRepo), kicker, nil)

	lsRepo.On("GetByID", mock.Anything, "ls1").Return(entity.Livestream{ID: "ls1", StreamKey: "sk1", IsLive: true}, nil)
	lsRepo.On("Update", mock.Anything, mock.MatchedBy(func(ls *entity.Livestream) bool { return ls.IsBanned && !ls.IsLive })).Return(nil)

	require.NoError(t, uc.BanStream(context.Background(), "ls1"))
	assert.Equal(t, []string{"sk1"}, kicker.kicked)
}

func TestBanVideo_SetsBannedStatus(t *testing.T) {
	vRepo := new(mockVideoRepo)
	uc := admin.New(new(mockLivestreamRepo), vRepo, new(mockUserRepo), new(mockWorkerRepo), new(mockAdminStatsRepo), nil, nil)

	vRepo.On("GetByID", mock.Anything, "v1").Return(entity.Video{ID: "v1", Status: entity.VideoStatusComplete, Visibility: entity.VideoVisibilityPublic}, nil)
	vRepo.On("SetStatus", mock.Anything, "v1", entity.VideoStatusBanned, mock.Anything).Return(nil)

	require.NoError(t, uc.BanVideo(context.Background(), "v1"))
	vRepo.AssertExpectations(t)
}
