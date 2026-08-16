package notification

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/events"
	"github.com/evrone/go-clean-template/internal/mapper"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/google/uuid"
)

type UseCase struct {
	repo       repo.NotificationRepo
	followRepo repo.FollowRepo
	notifHub   *events.NotificationHub
}

func New(r repo.NotificationRepo, fr repo.FollowRepo, notifHub *events.NotificationHub) *UseCase {
	return &UseCase{
		repo:       r,
		followRepo: fr,
		notifHub:   notifHub,
	}
}

func (u *UseCase) ListNotifications(ctx context.Context, userID string, page, limit int) (response.NotificationListResponse, error) {
	offset := (page - 1) * limit
	notifs, total, err := u.repo.ListByUserID(ctx, userID, limit, offset)
	if err != nil {
		return response.NotificationListResponse{}, fmt.Errorf("NotificationUseCase - ListNotifications: %w", err)
	}

	unread, _ := u.repo.CountUnread(ctx, userID)

	return mapper.ToNotificationListResponse(notifs, total, unread, page, limit), nil
}

func (u *UseCase) MarkAsRead(ctx context.Context, id, userID string) error {
	if err := u.repo.MarkAsRead(ctx, id, userID); err != nil {
		return fmt.Errorf("NotificationUseCase - MarkAsRead: %w", err)
	}
	return nil
}

func (u *UseCase) SubscribeNotifications(userID string) (<-chan response.NotificationResponse, func(), error) {
	if u.notifHub == nil {
		return nil, nil, errors.New("notification hub unavailable")
	}
	rawChan, unsub := u.notifHub.Subscribe(userID)

	mapped := make(chan response.NotificationResponse)
	go func() {
		defer close(mapped)
		for notif := range rawChan {
			mapped <- mapper.ToNotificationResponse(notif)
		}
	}()

	return mapped, unsub, nil
}

// NotifyFollowers queries all followers of senderID and pushes a notification to each of them
func (u *UseCase) NotifyFollowers(
	ctx context.Context,
	senderID, senderName, senderAvatar string,
	notifType entity.NotificationType,
	title, message, targetURL string,
) error {
	if u.followRepo == nil {
		return nil
	}

	// Fetch up to 1000 followers
	followers, _, err := u.followRepo.ListFollowedChannels(ctx, senderID, 1, 1000)
	if err != nil && !errors.Is(err, entity.ErrUserNotFound) {
		// Try fetching via raw channel followers if needed, or iterate
	}

	// We can also query all followers directly via followRepo if available or list followers
	// For each follower, save to notification table & push to hub if online
	now := time.Now().UTC()
	for _, follower := range followers {
		notif := entity.Notification{
			ID:           uuid.New().String(),
			UserID:       follower.ID,
			SenderID:     senderID,
			SenderName:   senderName,
			SenderAvatar: senderAvatar,
			Type:         notifType,
			Title:        title,
			Message:      message,
			TargetURL:    targetURL,
			IsRead:       false,
			CreatedAt:    now,
		}

		_ = u.repo.Store(ctx, &notif)
		if u.notifHub != nil {
			u.notifHub.SendDirect(follower.ID, notif)
		}
	}

	return nil
}
