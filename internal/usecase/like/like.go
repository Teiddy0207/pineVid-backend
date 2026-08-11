package like

import (
	"context"
	"fmt"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/mapper"
	persistLikeRepo "github.com/evrone/go-clean-template/internal/repo/persistent/like"
	"github.com/evrone/go-clean-template/pkg/nats"
)

type UseCase struct {
	repo          *persistLikeRepo.Repo
	natsPublisher *nats.Publisher
}

func New(r *persistLikeRepo.Repo, natsPub *nats.Publisher) *UseCase {
	return &UseCase{
		repo:          r,
		natsPublisher: natsPub,
	}
}

func (u *UseCase) ToggleLikeVideo(ctx context.Context, userID, videoID string) (response.LikeResponse, error) {
	likeEntity := mapper.ToLikeEntity(userID, videoID)
	liked, totalLikes, err := u.repo.ToggleLike(ctx, &likeEntity)
	if err != nil {
		return response.LikeResponse{}, fmt.Errorf("LikeUseCase - ToggleLikeVideo: %w", err)
	}

	return mapper.ToLikeResponse(videoID, liked, totalLikes), nil
}

func (u *UseCase) HeartStream(ctx context.Context, streamID string) (response.HeartResponse, error) {
	totalHearts, err := u.repo.IncrementHeart(ctx, streamID)
	if err != nil {
		return response.HeartResponse{}, fmt.Errorf("LikeUseCase - HeartStream: %w", err)
	}

	return mapper.ToHeartResponse(streamID, totalHearts), nil
}
