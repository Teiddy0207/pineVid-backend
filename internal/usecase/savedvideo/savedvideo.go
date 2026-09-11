package savedvideo

import (
	"context"
	"fmt"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/mapper"
	"github.com/evrone/go-clean-template/internal/repo"
)

type UseCase struct {
	repo repo.SavedVideoRepo
}

func New(r repo.SavedVideoRepo) *UseCase {
	return &UseCase{repo: r}
}

func (u *UseCase) ToggleSaveVideo(ctx context.Context, userID, videoID string) (response.SaveVideoResponse, error) {
	saved, err := u.repo.ToggleSave(ctx, userID, videoID)
	if err != nil {
		return response.SaveVideoResponse{}, fmt.Errorf("SavedVideoUseCase - ToggleSaveVideo: %w", err)
	}
	return response.SaveVideoResponse{VideoID: videoID, Saved: saved}, nil
}

func (u *UseCase) ListSavedVideos(ctx context.Context, userID string, page, limit int) (response.PageResponse[response.VideoResponse], error) {
	offset := (page - 1) * limit
	videos, total, err := u.repo.ListSavedVideos(ctx, userID, limit, offset)
	if err != nil {
		return response.PageResponse[response.VideoResponse]{}, fmt.Errorf("SavedVideoUseCase - ListSavedVideos: %w", err)
	}
	return mapper.ToVideoPageResponse(videos, total, page, limit), nil
}
