package like

import (
	"context"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/stretchr/testify/require"
	"testing"
)

type roomStub struct {
	usecase.Livestream
	live       bool
	err        error
	broadcasts int
}

func (s *roomStub) GetStreamByID(context.Context, string) (response.LivestreamResponse, error) {
	return response.LivestreamResponse{IsLive: s.live}, s.err
}
func (s *roomStub) BroadcastHeart(string, int64) { s.broadcasts++ }

type heartRepo struct {
	repo.LikeRepo
	calls int
}

func (s *heartRepo) IncrementHeart(context.Context, string) (int64, error) { s.calls++; return 5, nil }
func TestHeartsRequireActiveRoom(t *testing.T) {
	for _, tc := range []struct {
		name           string
		live           bool
		err, errorWant error
	}{
		{"offline", false, nil, entity.ErrLivestreamNotLive},
		{"missing", false, entity.ErrLivestreamNotFound, entity.ErrLivestreamNotFound},
		{"live", true, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rooms := &roomStub{live: tc.live, err: tc.err}
			counter := &heartRepo{}
			uc := New(counter, nil, rooms)
			result, err := uc.HeartStream(context.Background(), "room", "")
			if tc.errorWant != nil {
				require.ErrorIs(t, err, tc.errorWant)
				require.Zero(t, counter.calls)
				require.Zero(t, rooms.broadcasts)
			} else {
				require.NoError(t, err)
				require.EqualValues(t, 5, result.TotalHearts)
				require.Equal(t, 1, counter.calls)
				require.Equal(t, 1, rooms.broadcasts)
			}
		})
	}
}
