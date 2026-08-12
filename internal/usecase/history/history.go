package history

import (
	"context"
	"fmt"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/mapper"
	"github.com/evrone/go-clean-template/internal/repo"
)

type UseCase struct {
	repo repo.WatchHistoryRepo
}

func New(r repo.WatchHistoryRepo) *UseCase {
	return &UseCase{repo: r}
}

// RecordWatch upserts the user's watch progress for a video. Best-effort from
// the caller's perspective: a failure here should never fail the surrounding
// request (e.g. the public view-count endpoint that triggers it).
func (u *UseCase) RecordWatch(ctx context.Context, userID, videoID string, watchSeconds int) error {
	if userID == "" || videoID == "" {
		return nil
	}
	if err := u.repo.Upsert(ctx, userID, videoID, watchSeconds); err != nil {
		return fmt.Errorf("HistoryUseCase - RecordWatch - Upsert: %w", err)
	}
	return nil
}

func (u *UseCase) ListHistory(ctx context.Context, userID string, page, limit int) (response.PageResponse[response.VideoResponse], error) {
	offset := (page - 1) * limit
	videos, total, err := u.repo.ListByUser(ctx, userID, limit, offset)
	if err != nil {
		return response.PageResponse[response.VideoResponse]{}, fmt.Errorf("HistoryUseCase - ListHistory: %w", err)
	}
	return mapper.ToVideoPageResponse(videos, total, page, limit), nil
}
