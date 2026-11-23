package video_test

import (
	"context"
	"errors"
	"testing"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase/video"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakeMedia struct {
	prefixes []string
	objects  []string
	err      error
}

func (f *fakeMedia) RemovePrefix(_ context.Context, bucket, prefix string) (int, error) {
	f.prefixes = append(f.prefixes, bucket+"/"+prefix)
	return 3, f.err
}

func (f *fakeMedia) RemoveObject(_ context.Context, bucket, key string) error {
	f.objects = append(f.objects, bucket+"/"+key)
	return f.err
}

func TestDeleteVideo_RemovesMediaAfterRow(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	v := entity.Video{ID: "v1", UserID: "owner", RawS3Key: "raw-uploads/v1/raw.mp4"}
	repo.On("GetByID", mock.Anything, "v1").Return(v, nil)
	repo.On("Delete", mock.Anything, "v1").Return(nil)
	media := &fakeMedia{}
	uc := video.New(repo, nil, nil, nil, nil, nil).WithMediaRemover(media, "raw-videos", "hls-streams", nil)

	require.NoError(t, uc.DeleteVideo(context.Background(), "owner", "v1"))
	assert.Equal(t, []string{"hls-streams/videos/v1/"}, media.prefixes)
	assert.Equal(t, []string{"raw-videos/raw-uploads/v1/raw.mp4"}, media.objects)
}

func TestDeleteVideo_OtherUserTouchesNothing(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	repo.On("GetByID", mock.Anything, "v1").Return(entity.Video{ID: "v1", UserID: "owner"}, nil)
	media := &fakeMedia{}
	uc := video.New(repo, nil, nil, nil, nil, nil).WithMediaRemover(media, "raw-videos", "hls-streams", nil)

	require.ErrorIs(t, uc.DeleteVideo(context.Background(), "intruder", "v1"), entity.ErrVideoForbidden)
	repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
	assert.Empty(t, media.prefixes)
	assert.Empty(t, media.objects)
}

func TestDeleteVideo_CleanupFailureIsReportedNotReturned(t *testing.T) {
	t.Parallel()

	repo := new(MockVideoRepo)
	repo.On("GetByID", mock.Anything, "v1").Return(entity.Video{ID: "v1", UserID: "owner", RawS3Key: "k"}, nil)
	repo.On("Delete", mock.Anything, "v1").Return(nil)
	var reported []error
	media := &fakeMedia{err: errors.New("minio down")}
	uc := video.New(repo, nil, nil, nil, nil, nil).WithMediaRemover(media, "raw-videos", "hls-streams", func(err error) { reported = append(reported, err) })

	require.NoError(t, uc.DeleteVideo(context.Background(), "owner", "v1"), "the row is gone, so the request succeeded")
	assert.Len(t, reported, 2)
}
