package comment_test

import (
	"context"
	"testing"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase/comment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockCommentRepo is a hand-rolled testify mock implementing repo.CommentRepo,
// following the same local-mock convention as
// internal/usecase/video/transcode_callback_test.go's MockVideoRepo.
type MockCommentRepo struct {
	mock.Mock
}

func (m *MockCommentRepo) Store(ctx context.Context, c *entity.Comment) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}

func (m *MockCommentRepo) GetByID(ctx context.Context, id string) (entity.Comment, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(entity.Comment), args.Error(1)
}

func (m *MockCommentRepo) ListByVideoID(ctx context.Context, videoID string, limit, offset uint64) ([]entity.Comment, int, error) {
	args := m.Called(ctx, videoID, limit, offset)
	return args.Get(0).([]entity.Comment), args.Int(1), args.Error(2)
}

func (m *MockCommentRepo) ListRepliesByParentID(ctx context.Context, parentID string, limit, offset uint64) ([]entity.Comment, int, error) {
	args := m.Called(ctx, parentID, limit, offset)
	return args.Get(0).([]entity.Comment), args.Int(1), args.Error(2)
}

func (m *MockCommentRepo) CountAllByVideoID(ctx context.Context, videoID string) (int, error) {
	args := m.Called(ctx, videoID)
	return args.Int(0), args.Error(1)
}

// MockCommentLikeRepo is a hand-rolled testify mock implementing repo.CommentLikeRepo.
type MockCommentLikeRepo struct {
	mock.Mock
}

func (m *MockCommentLikeRepo) ToggleLike(ctx context.Context, commentID, userID string) (bool, int64, error) {
	args := m.Called(ctx, commentID, userID)
	return args.Bool(0), args.Get(1).(int64), args.Error(2)
}

func (m *MockCommentLikeRepo) GetLikeCount(ctx context.Context, commentID string) (int64, error) {
	args := m.Called(ctx, commentID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockCommentLikeRepo) IsLikedByUser(ctx context.Context, commentID, userID string) (bool, error) {
	args := m.Called(ctx, commentID, userID)
	return args.Bool(0), args.Error(1)
}

func TestCreateComment_TopLevel(t *testing.T) {
	t.Parallel()

	repo := new(MockCommentRepo)
	uc := comment.New(repo, nil, nil)

	repo.On("Store", mock.Anything, mock.MatchedBy(func(c *entity.Comment) bool {
		return c.VideoID == "video-1" && c.ParentID == nil
	})).Return(nil)

	res, err := uc.CreateComment(context.Background(), "video-1", "user-1", "Alice", "", request.CreateCommentRequest{
		Content: "nice video",
	})

	require.NoError(t, err)
	assert.Equal(t, "video-1", res.VideoID)
	assert.Nil(t, res.ParentID)
}

func TestCreateComment_ValidReply(t *testing.T) {
	t.Parallel()

	repo := new(MockCommentRepo)
	uc := comment.New(repo, nil, nil)

	parentID := "parent-1"
	repo.On("GetByID", mock.Anything, parentID).Return(entity.Comment{ID: parentID, VideoID: "video-1"}, nil)
	repo.On("Store", mock.Anything, mock.MatchedBy(func(c *entity.Comment) bool {
		return c.ParentID != nil && *c.ParentID == parentID
	})).Return(nil)

	res, err := uc.CreateComment(context.Background(), "video-1", "user-2", "Bob", "", request.CreateCommentRequest{
		Content:  "totally agree",
		ParentID: &parentID,
	})

	require.NoError(t, err)
	require.NotNil(t, res.ParentID)
	assert.Equal(t, parentID, *res.ParentID)
}

func TestCreateComment_ReplyToMissingParent(t *testing.T) {
	t.Parallel()

	repo := new(MockCommentRepo)
	uc := comment.New(repo, nil, nil)

	parentID := "does-not-exist"
	repo.On("GetByID", mock.Anything, parentID).Return(entity.Comment{}, entity.ErrCommentNotFound)

	_, err := uc.CreateComment(context.Background(), "video-1", "user-2", "Bob", "", request.CreateCommentRequest{
		Content:  "reply to nothing",
		ParentID: &parentID,
	})

	require.ErrorIs(t, err, entity.ErrInvalidParentID)
	repo.AssertNotCalled(t, "Store", mock.Anything, mock.Anything)
}

func TestCreateComment_ReplyToCommentFromDifferentVideo(t *testing.T) {
	t.Parallel()

	repo := new(MockCommentRepo)
	uc := comment.New(repo, nil, nil)

	parentID := "parent-on-other-video"
	repo.On("GetByID", mock.Anything, parentID).Return(entity.Comment{ID: parentID, VideoID: "other-video"}, nil)

	_, err := uc.CreateComment(context.Background(), "video-1", "user-2", "Bob", "", request.CreateCommentRequest{
		Content:  "cross-video reply",
		ParentID: &parentID,
	})

	require.ErrorIs(t, err, entity.ErrInvalidParentID)
	repo.AssertNotCalled(t, "Store", mock.Anything, mock.Anything)
}

func TestListReplies(t *testing.T) {
	t.Parallel()

	repo := new(MockCommentRepo)
	uc := comment.New(repo, nil, nil)

	replies := []entity.Comment{
		{ID: "r1", VideoID: "video-1"},
		{ID: "r2", VideoID: "video-1"},
	}
	repo.On("ListRepliesByParentID", mock.Anything, "parent-1", uint64(10), uint64(0)).Return(replies, 2, nil)

	res, err := uc.ListReplies(context.Background(), "parent-1", "", 1, 10)

	require.NoError(t, err)
	assert.Len(t, res.Data, 2)
	assert.Equal(t, 2, res.Pagination.TotalItems)
}

func TestListVideoComments_TotalAllCountIncludesReplies(t *testing.T) {
	t.Parallel()

	repo := new(MockCommentRepo)
	uc := comment.New(repo, nil, nil)

	topLevel := []entity.Comment{{ID: "c1", VideoID: "video-1"}}
	repo.On("ListByVideoID", mock.Anything, "video-1", uint64(10), uint64(0)).Return(topLevel, 1, nil)
	// 1 top-level comment but 3 replies scattered under it/other comments —
	// TotalAllCount must reflect all 4, not the paginated top-level total of 1.
	repo.On("CountAllByVideoID", mock.Anything, "video-1").Return(4, nil)

	res, err := uc.ListVideoComments(context.Background(), "video-1", "", 1, 10)

	require.NoError(t, err)
	assert.Equal(t, 1, res.Pagination.TotalItems)
	assert.Equal(t, 4, res.TotalAllCount)
}

func TestToggleLikeComment(t *testing.T) {
	t.Parallel()

	repo := new(MockCommentRepo)
	likeRepo := new(MockCommentLikeRepo)
	uc := comment.New(repo, likeRepo, nil)

	likeRepo.On("ToggleLike", mock.Anything, "c1", "user-1").Return(true, int64(5), nil)

	res, err := uc.ToggleLikeComment(context.Background(), "c1", "user-1")

	require.NoError(t, err)
	assert.Equal(t, "c1", res.CommentID)
	assert.True(t, res.Liked)
	assert.Equal(t, int64(5), res.TotalLikes)
}

func TestListVideoComments_PopulatesLikeInfo(t *testing.T) {
	t.Parallel()

	repo := new(MockCommentRepo)
	likeRepo := new(MockCommentLikeRepo)
	uc := comment.New(repo, likeRepo, nil)

	topLevel := []entity.Comment{{ID: "c1", VideoID: "video-1"}}
	repo.On("ListByVideoID", mock.Anything, "video-1", uint64(10), uint64(0)).Return(topLevel, 1, nil)
	repo.On("CountAllByVideoID", mock.Anything, "video-1").Return(1, nil)
	likeRepo.On("GetLikeCount", mock.Anything, "c1").Return(int64(3), nil)
	likeRepo.On("IsLikedByUser", mock.Anything, "c1", "viewer-1").Return(true, nil)

	res, err := uc.ListVideoComments(context.Background(), "video-1", "viewer-1", 1, 10)

	require.NoError(t, err)
	require.Len(t, res.Data, 1)
	assert.Equal(t, int64(3), res.Data[0].LikeCount)
	assert.True(t, res.Data[0].IsLiked)
}
