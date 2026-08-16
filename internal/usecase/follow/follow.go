package follow

import (
	"context"
	"fmt"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
)

type UseCase struct {
	repo repo.FollowRepo
}

func New(r repo.FollowRepo) *UseCase {
	return &UseCase{repo: r}
}

// ToggleFollow follows channelID if followerID doesn't already follow it, or
// unfollows it otherwise, returning the resulting state and fresh follower count.
func (u *UseCase) ToggleFollow(ctx context.Context, followerID, channelID string) (response.FollowToggleResponse, error) {
	if followerID == channelID {
		return response.FollowToggleResponse{}, entity.ErrCannotFollowSelf
	}

	following, err := u.repo.IsFollowing(ctx, followerID, channelID)
	if err != nil {
		return response.FollowToggleResponse{}, fmt.Errorf("FollowUseCase - ToggleFollow - IsFollowing: %w", err)
	}

	if following {
		if err := u.repo.Unfollow(ctx, followerID, channelID); err != nil {
			return response.FollowToggleResponse{}, fmt.Errorf("FollowUseCase - ToggleFollow - Unfollow: %w", err)
		}
	} else {
		if err := u.repo.Follow(ctx, followerID, channelID); err != nil {
			return response.FollowToggleResponse{}, fmt.Errorf("FollowUseCase - ToggleFollow - Follow: %w", err)
		}
	}

	count, err := u.repo.CountFollowers(ctx, channelID)
	if err != nil {
		return response.FollowToggleResponse{}, fmt.Errorf("FollowUseCase - ToggleFollow - CountFollowers: %w", err)
	}

	return response.FollowToggleResponse{
		ChannelID:      channelID,
		Following:      !following,
		FollowersCount: count,
	}, nil
}

func (u *UseCase) CountFollowers(ctx context.Context, channelID string) (int64, error) {
	return u.repo.CountFollowers(ctx, channelID)
}

func (u *UseCase) IsFollowing(ctx context.Context, followerID, channelID string) (bool, error) {
	if followerID == "" {
		return false, nil
	}
	return u.repo.IsFollowing(ctx, followerID, channelID)
}

func (u *UseCase) ListFollowedChannels(ctx context.Context, followerID string, page, limit int) (response.PageResponse[response.ChannelSummary], error) {
	channels, total, err := u.repo.ListFollowedChannels(ctx, followerID, page, limit)
	if err != nil {
		return response.PageResponse[response.ChannelSummary]{}, fmt.Errorf("FollowUseCase - ListFollowedChannels: %w", err)
	}

	items := make([]response.ChannelSummary, len(channels))
	for i, c := range channels {
		items[i] = response.ChannelSummary{ID: c.ID, Username: c.Username, Avatar: c.Avatar}
	}

	totalPages := 0
	if limit > 0 {
		totalPages = (total + limit - 1) / limit
	}

	return response.PageResponse[response.ChannelSummary]{
		Success: true,
		Data:    items,
		Pagination: response.PaginationMeta{
			TotalItems:  total,
			TotalPages:  totalPages,
			CurrentPage: page,
			Limit:       limit,
		},
	}, nil
}
