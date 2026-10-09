package video_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo/persistent/view"
	"github.com/evrone/go-clean-template/internal/usecase/video"
	"github.com/evrone/go-clean-template/pkg/nats"
	redispkg "github.com/evrone/go-clean-template/pkg/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakeViewPublisher struct{ got []nats.ViewUpdated }

func (f *fakeViewPublisher) PublishViewUpdated(ev nats.ViewUpdated) error {
	f.got = append(f.got, ev)
	return nil
}

type viewFixture struct {
	uc   *video.UseCase
	repo *MockVideoRepo
	mr   *miniredis.Miniredis
	pub  *fakeViewPublisher
}

func newViewFixture(t *testing.T, v entity.Video) viewFixture {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb, err := redispkg.New(mr.Addr(), "", 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rdb.Close() })

	repo := new(MockVideoRepo)
	repo.On("GetByID", mock.Anything, v.ID).Return(v, nil)
	pub := &fakeViewPublisher{}
	uc := video.New(repo, view.New(nil, rdb), nil, nil, nil, nil).WithViewPublisher(pub)
	return viewFixture{uc: uc, repo: repo, mr: mr, pub: pub}
}

// ticket plants a view ticket issued `age` ago (so the 10 s rule can be
// exercised without waiting).
func (f viewFixture) ticket(t *testing.T, videoID string, age time.Duration) string {
	t.Helper()
	tk := fmt.Sprintf("tk-%d", time.Now().UnixNano())
	require.NoError(t, f.mr.Set("view_ticket:"+tk, fmt.Sprintf("%s|%d", videoID, time.Now().Add(-age).UnixMilli())))
	return tk
}

var publicVideo = entity.Video{ID: "v1", UserID: "owner", Status: entity.VideoStatusComplete, Visibility: entity.VideoVisibilityPublic, Views: 100}

func TestRecordView_TotalIncludesSyncedViewsAndIsPublished(t *testing.T) {
	t.Parallel()
	f := newViewFixture(t, publicVideo)
	ctx := context.Background()

	recorded, total, err := f.uc.RecordView(ctx, "v1", f.ticket(t, "v1", 11*time.Second), "", "1.2.3.4", "UA")
	require.NoError(t, err)
	assert.True(t, recorded)
	assert.Equal(t, int64(101), total, "100 synced in PostgreSQL + 1 pending in Redis")

	require.Len(t, f.pub.got, 1)
	assert.Equal(t, "v1", f.pub.got[0].VideoID)
	assert.Equal(t, int64(101), f.pub.got[0].TotalViews)
	assert.InDelta(t, time.Now().UnixMilli(), f.pub.got[0].TS, 5000)

	// Same viewer again within 60 s: not counted, not announced.
	recorded, total, err = f.uc.RecordView(ctx, "v1", f.ticket(t, "v1", 11*time.Second), "", "1.2.3.4", "UA")
	require.NoError(t, err)
	assert.False(t, recorded)
	assert.Equal(t, int64(101), total)
	assert.Len(t, f.pub.got, 1)
}

func TestRecordView_TicketRules(t *testing.T) {
	t.Parallel()
	f := newViewFixture(t, publicVideo)
	ctx := context.Background()

	_, _, err := f.uc.RecordView(ctx, "v1", "", "", "1.2.3.4", "UA")
	assert.ErrorIs(t, err, entity.ErrViewTicketInvalid, "no ticket")

	_, _, err = f.uc.RecordView(ctx, "v1", f.ticket(t, "v1", 3*time.Second), "", "1.2.3.4", "UA")
	assert.ErrorIs(t, err, entity.ErrViewTicketTooEarly, "watched only 3 s")

	_, _, err = f.uc.RecordView(ctx, "v1", f.ticket(t, "other-video", time.Minute), "", "1.2.3.4", "UA")
	assert.ErrorIs(t, err, entity.ErrViewTicketInvalid, "ticket of another video")

	assert.Empty(t, f.pub.got)
}

// A private video answers "not found" before the ticket is even looked at.
func TestRecordView_PrivateVideoIsNotFound(t *testing.T) {
	t.Parallel()
	private := publicVideo
	private.Visibility = entity.VideoVisibilityPrivate
	f := newViewFixture(t, private)

	_, _, err := f.uc.RecordView(context.Background(), "v1", "", "", "1.2.3.4", "UA")
	assert.ErrorIs(t, err, entity.ErrVideoNotFound)

	total, err := f.uc.CurrentViews(context.Background(), "v1", "")
	assert.ErrorIs(t, err, entity.ErrVideoNotFound)
	assert.Zero(t, total)

	total, err = f.uc.CurrentViews(context.Background(), "v1", "owner")
	require.NoError(t, err, "the owner may watch it")
	assert.Equal(t, int64(100), total)
}

func TestRecordView_LoggedInViewerDedupsAcrossDevices(t *testing.T) {
	t.Parallel()
	f := newViewFixture(t, publicVideo)
	ctx := context.Background()

	recorded, _, err := f.uc.RecordView(ctx, "v1", f.ticket(t, "v1", 11*time.Second), "user-7", "1.2.3.4", "Firefox")
	require.NoError(t, err)
	assert.True(t, recorded)

	recorded, _, err = f.uc.RecordView(ctx, "v1", f.ticket(t, "v1", 11*time.Second), "user-7", "5.6.7.8", "Chrome")
	require.NoError(t, err)
	assert.False(t, recorded, "same account on another device within 60 s")
}
