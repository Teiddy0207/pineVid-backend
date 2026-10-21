package like

import (
	"context"
	"fmt"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/mapper"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/nats"
)

type UseCase struct {
	repo          repo.LikeRepo
	liveEvents    repo.LiveEventLog
	natsPublisher *nats.Publisher
	livestreamUc  usecase.Livestream
}

func New(r repo.LikeRepo, natsPub *nats.Publisher, livestreamUc usecase.Livestream) *UseCase {
	return &UseCase{
		repo:          r,
		natsPublisher: natsPub,
		livestreamUc:  livestreamUc,
	}
}

// WithLiveEvents records every heart on the live events stream (aggregated
// per broadcast by live-agg) instead of only bumping a Redis counter.
func (u *UseCase) WithLiveEvents(l repo.LiveEventLog) *UseCase {
	u.liveEvents = l
	return u
}

func (u *UseCase) ToggleLikeVideo(ctx context.Context, userID, videoID string) (response.LikeResponse, error) {
	likeEntity := mapper.ToLikeEntity(userID, videoID)
	liked, totalLikes, err := u.repo.ToggleLike(ctx, &likeEntity)
	if err != nil {
		return response.LikeResponse{}, fmt.Errorf("LikeUseCase - ToggleLikeVideo: %w", err)
	}

	return mapper.ToLikeResponse(videoID, liked, totalLikes), nil
}

func (u *UseCase) HeartStream(ctx context.Context, streamID, userID string) (response.HeartResponse, error) {
	var (
		totalHearts int64
		err         error
	)
	if u.liveEvents != nil {
		totalHearts, err = u.liveEvents.AppendHeart(ctx, entity.LiveEvent{
			Type:         entity.LiveEventHeart,
			LivestreamID: streamID,
			UserID:       userID,
			At:           time.Now(),
		})
	} else {
		totalHearts, err = u.repo.IncrementHeart(ctx, streamID)
	}
	if err != nil {
		return response.HeartResponse{}, fmt.Errorf("LikeUseCase - HeartStream: %w", err)
	}

	// Broadcast to every viewer in the room in real time — the caller only
	// gets their own HTTP response back, but everyone else needs to see the
	// heart burst too for the room to feel alive.
	if u.livestreamUc != nil {
		u.livestreamUc.BroadcastHeart(streamID, totalHearts)
	}

	return mapper.ToHeartResponse(streamID, totalHearts), nil
}
