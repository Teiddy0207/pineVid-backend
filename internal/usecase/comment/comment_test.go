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

func TestCreateComment_TopLevel(t *testing.T) {
	t.Parallel()

	repo := new(MockCommentRepo)
	uc := comment.New(repo, nil)

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
	uc := comment.New(repo, nil)

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
	uc := comment.New(repo, nil)

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
	uc := comment.New(repo, nil)

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
	uc := comment.New(repo, nil)

	replies := []entity.Comment{
		{ID: "r1", VideoID: "video-1"},
		{ID: "r2", VideoID: "video-1"},
	}
	repo.On("ListRepliesByParentID", mock.Anything, "parent-1", uint64(10), uint64(0)).Return(replies, 2, nil)

	res, err := uc.ListReplies(context.Background(), "parent-1", 1, 10)

	require.NoError(t, err)
	assert.Len(t, res.Data, 2)
	assert.Equal(t, 2, res.Pagination.TotalItems)
}
