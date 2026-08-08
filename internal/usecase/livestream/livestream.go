package livestream

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/mapper"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/google/uuid"
)

type UseCase struct {
	repo repo.LivestreamRepo
}

func New(r repo.LivestreamRepo) *UseCase {
	return &UseCase{repo: r}
}

func (u *UseCase) GetStreamKey(ctx context.Context, userID string) (response.StreamKeyResponse, error) {
	ls, err := u.repo.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, entity.ErrLivestreamNotFound) {
			// Auto create livestream record for user if not exists
			newStreamKey := fmt.Sprintf("sk_live_%s", uuid.New().String()[:18])
			newLs := entity.Livestream{
				ID:        uuid.New().String(),
				UserID:    userID,
				StreamKey: newStreamKey,
				Title:     "My Livestream",
				Category:  "General",
				IsLive:    false,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}
			if err := u.repo.Store(ctx, &newLs); err != nil {
				return response.StreamKeyResponse{}, fmt.Errorf("LivestreamUseCase - GetStreamKey - Store: %w", err)
			}
			ls = newLs
		} else {
			return response.StreamKeyResponse{}, err
		}
	}

	return response.StreamKeyResponse{
		ServerUrl: "rtmp://live.pipevid.com/live",
		StreamKey: ls.StreamKey,
	}, nil
}

func (u *UseCase) ResetStreamKey(ctx context.Context, userID string) (response.StreamKeyResponse, error) {
	ls, err := u.repo.GetByUserID(ctx, userID)
	if err != nil {
		return response.StreamKeyResponse{}, err
	}

	ls.StreamKey = fmt.Sprintf("sk_live_%s", uuid.New().String()[:18])
	ls.UpdatedAt = time.Now().UTC()

	if err := u.repo.Update(ctx, &ls); err != nil {
		return response.StreamKeyResponse{}, fmt.Errorf("LivestreamUseCase - ResetStreamKey - Update: %w", err)
	}

	return response.StreamKeyResponse{
		ServerUrl: "rtmp://live.pipevid.com/live",
		StreamKey: ls.StreamKey,
	}, nil
}

func (u *UseCase) AuthenticateStreamKey(ctx context.Context, req request.StreamKeyAuth) (bool, error) {
	ls, err := u.repo.GetByStreamKey(ctx, req.StreamKey)
	if err != nil {
		return false, entity.ErrInvalidStreamKey
	}

	// Mark stream as live
	now := time.Now().UTC()
	ls.IsLive = true
	ls.StartedAt = &now
	ls.HLSUrl = fmt.Sprintf("https://s3.pipevid.com/live/%s/master.m3u8", ls.StreamKey)
	ls.UpdatedAt = now

	_ = u.repo.Update(ctx, &ls)

	return true, nil
}

func (u *UseCase) GetStreamByID(ctx context.Context, id string) (response.LivestreamResponse, error) {
	ls, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return response.LivestreamResponse{}, err
	}
	return mapper.ToLivestreamResponse(ls), nil
}

func (u *UseCase) ListActiveStreams(ctx context.Context, category string, page, limit int) (response.PageResponse[response.LivestreamResponse], error) {
	offset := (page - 1) * limit
	streams, total, err := u.repo.ListActive(ctx, category, limit, offset)
	if err != nil {
		return response.PageResponse[response.LivestreamResponse]{}, err
	}

	return mapper.ToLivestreamPageResponse(streams, total, page, limit), nil
}

func (u *UseCase) UpdateStreamInfo(ctx context.Context, userID string, req request.UpdateLivestreamInfo) (response.LivestreamResponse, error) {
	ls, err := u.repo.GetByUserID(ctx, userID)
	if err != nil {
		return response.LivestreamResponse{}, err
	}

	if req.Title != "" {
		ls.Title = req.Title
	}
	if req.Category != "" {
		ls.Category = req.Category
	}
	ls.UpdatedAt = time.Now().UTC()

	if err := u.repo.Update(ctx, &ls); err != nil {
		return response.LivestreamResponse{}, fmt.Errorf("LivestreamUseCase - UpdateStreamInfo - Update: %w", err)
	}

	return mapper.ToLivestreamResponse(ls), nil
}
