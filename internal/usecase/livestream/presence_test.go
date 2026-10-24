package livestream_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase/livestream"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakePresence struct {
	keys  []string
	count int64
}

func (f *fakePresence) Apply(_ context.Context, key, action, tab, who string) (int64, error) {
	f.keys = append(f.keys, key)
	return f.count, nil
}
func TestPresenceBroadcastBoundary(t *testing.T) {
	now := time.Now().UTC()
	repo := new(MockLivestreamRepo)
	store := &fakePresence{count: 2}
	repo.On("GetByID", mock.Anything, "live").Return(entity.Livestream{ID: "live", IsLive: true, StartedAt: &now}, nil)
	uc := newLivestreamUseCase(repo).WithPresence(store)
	first, err := uc.ViewerPresence(context.Background(), "live", "heartbeat", "tab", "user", "")
	require.NoError(t, err)
	require.EqualValues(t, 2, first.ViewersCount)
	_, err = uc.ViewerPresence(context.Background(), "live", "leave", "tab", "user", "old-broadcast")
	require.ErrorIs(t, err, livestream.ErrStaleBroadcast)
	require.Len(t, store.keys, 1)
	response, err := uc.GetStreamByID(context.Background(), "live")
	require.NoError(t, err)
	require.EqualValues(t, 2, response.ViewersCount)
}
func TestPresenceRejectsOfflineBannedAndMissingRedis(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		row  entity.Livestream
		want error
	}{
		{entity.Livestream{}, livestream.ErrStreamOffline},
		{entity.Livestream{IsBanned: true}, entity.ErrStreamBanned},
		{entity.Livestream{OwnerBanned: true}, entity.ErrStreamBanned},
		{entity.Livestream{IsLive: true, StartedAt: &now}, livestream.ErrPresenceUnavailable},
	} {
		repo := new(MockLivestreamRepo)
		repo.On("GetByID", mock.Anything, "live").Return(tc.row, nil)
		_, err := newLivestreamUseCase(repo).ViewerPresence(context.Background(), "live", "count", "", "", "")
		require.True(t, errors.Is(err, tc.want), "got %v want %v", err, tc.want)
	}
}
