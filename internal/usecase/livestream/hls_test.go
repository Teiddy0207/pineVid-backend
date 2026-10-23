package livestream_test

import (
	"context"
	"errors"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/stretchr/testify/mock"
	"testing"
)

func TestLiveMasterRejectsBannedBeforeProbing(t *testing.T) {
	for _, ls := range []entity.Livestream{{IsLive: true, IsBanned: true}, {IsLive: true, OwnerBanned: true}} {
		repo := new(MockLivestreamRepo)
		repo.On("GetByID", mock.Anything, "id").Return(ls, nil)
		u := newLivestreamUseCase(repo).WithLiveHLS(true)
		if _, err := u.LiveMaster(context.Background(), "id"); !errors.Is(err, entity.ErrStreamBanned) {
			t.Fatal(err)
		}
		repo.AssertExpectations(t)
	}
}
func TestDerivedPublishCannotCreateSession(t *testing.T) {
	u := newLivestreamUseCase(new(MockLivestreamRepo))
	ok, err := u.AuthenticateStreamKey(context.Background(), request.StreamKeyAuth{App: "abr", StreamKey: "key"})
	if ok || !errors.Is(err, entity.ErrInvalidStreamKey) {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}
