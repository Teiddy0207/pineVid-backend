package post

import (
	"context"
	"errors"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/stretchr/testify/require"
	"testing"
)

type postStub struct {
	repo.PostRepo
	err error
}

func (s postStub) GetByID(context.Context, string) (entity.Post, error) {
	return entity.Post{ID: "post"}, s.err
}
func (s postStub) ListByUser(context.Context, string, uint64, uint64) ([]entity.Post, int, error) {
	return []entity.Post{{ID: "post"}}, 1, nil
}

type postLikeStub struct {
	repo.PostLikeRepo
	calls              int
	liked              bool
	count              int64
	countErr, stateErr error
}

func (s *postLikeStub) ToggleLike(context.Context, string, string) (bool, int64, error) {
	s.calls++
	return s.liked, s.count, nil
}
func (s *postLikeStub) GetLikeCount(context.Context, string) (int64, error) {
	return s.count, s.countErr
}
func (s *postLikeStub) IsLikedByUser(context.Context, string, string) (bool, error) {
	return s.liked, s.stateErr
}
func TestLikeMissingPostDoesNotMutate(t *testing.T) {
	likes := &postLikeStub{}
	uc := New(postStub{err: entity.ErrPostNotFound}, likes, nil)
	_, err := uc.TogglePostLike(context.Background(), "missing", "viewer")
	require.ErrorIs(t, err, entity.ErrPostNotFound)
	require.Zero(t, likes.calls)
}
func TestLikeReturnsAuthoritativeResult(t *testing.T) {
	for _, liked := range []bool{true, false} {
		likes := &postLikeStub{liked: liked, count: 3}
		uc := New(postStub{}, likes, nil)
		result, err := uc.TogglePostLike(context.Background(), "post", "viewer")
		require.NoError(t, err)
		require.Equal(t, liked, result.Liked)
		require.EqualValues(t, 3, result.TotalLikes)
	}
}
func TestListPostsDoesNotHideLikeLookupFailure(t *testing.T) {
	unavailable := errors.New("database unavailable")
	for _, likes := range []*postLikeStub{{countErr: unavailable}, {stateErr: unavailable}} {
		uc := New(postStub{}, likes, nil)
		_, err := uc.ListPostsByUser(context.Background(), "author", "viewer", 1, 50)
		require.ErrorIs(t, err, unavailable)
	}
}
func TestListPostsIncludesLikeInfo(t *testing.T) {
	uc := New(postStub{}, &postLikeStub{liked: true, count: 9}, nil)
	page, err := uc.ListPostsByUser(context.Background(), "author", "viewer", 1, 50)
	require.NoError(t, err)
	require.Len(t, page.Data, 1)
	require.True(t, page.Data[0].IsLiked)
	require.EqualValues(t, 9, page.Data[0].LikeCount)
}
