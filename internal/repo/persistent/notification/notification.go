package notification

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/pkg/postgres"
)

type Repository struct {
	*postgres.Postgres
}

func New(pg *postgres.Postgres) *Repository {
	return &Repository{pg}
}

func (r *Repository) Store(ctx context.Context, notif *entity.Notification) error {
	sql, args, err := r.Builder.
		Insert("notifications").
		Columns("id", "user_id", "sender_id", "sender_name", "sender_avatar", "type", "title", "message", "target_url", "is_read", "created_at").
		Values(notif.ID, notif.UserID, notif.SenderID, notif.SenderName, notif.SenderAvatar, notif.Type, notif.Title, notif.Message, notif.TargetURL, notif.IsRead, notif.CreatedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("NotificationRepo - Store - ToSql: %w", err)
	}

	_, err = r.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("NotificationRepo - Store - Exec: %w", err)
	}

	return nil
}

func (r *Repository) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]entity.Notification, int, error) {
	// Count unread or total
	countSql, countArgs, err := r.Builder.
		Select("COUNT(*)").
		From("notifications").
		Where(squirrel.Eq{"user_id": userID}).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("NotificationRepo - ListByUserID - Count ToSql: %w", err)
	}

	var total int
	if err := r.Pool.QueryRow(ctx, countSql, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("NotificationRepo - ListByUserID - Count QueryRow: %w", err)
	}

	if total == 0 {
		return []entity.Notification{}, 0, nil
	}

	sql, args, err := r.Builder.
		Select("id", "user_id", "sender_id", "sender_name", "sender_avatar", "type", "title", "message", "target_url", "is_read", "created_at").
		From("notifications").
		Where(squirrel.Eq{"user_id": userID}).
		OrderBy("created_at DESC").
		Limit(uint64(limit)).
		Offset(uint64(offset)).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("NotificationRepo - ListByUserID - ToSql: %w", err)
	}

	rows, err := r.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("NotificationRepo - ListByUserID - Query: %w", err)
	}
	defer rows.Close()

	var notifs []entity.Notification
	for rows.Next() {
		var n entity.Notification
		err := rows.Scan(
			&n.ID, &n.UserID, &n.SenderID, &n.SenderName, &n.SenderAvatar,
			&n.Type, &n.Title, &n.Message, &n.TargetURL, &n.IsRead, &n.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("NotificationRepo - ListByUserID - Scan: %w", err)
		}
		notifs = append(notifs, n)
	}

	return notifs, total, nil
}

func (r *Repository) MarkAsRead(ctx context.Context, id, userID string) error {
	sql, args, err := r.Builder.
		Update("notifications").
		Set("is_read", true).
		Where(squirrel.Eq{"id": id, "user_id": userID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("NotificationRepo - MarkAsRead - ToSql: %w", err)
	}

	_, err = r.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("NotificationRepo - MarkAsRead - Exec: %w", err)
	}

	return nil
}

func (r *Repository) CountUnread(ctx context.Context, userID string) (int, error) {
	sql, args, err := r.Builder.
		Select("COUNT(*)").
		From("notifications").
		Where(squirrel.Eq{"user_id": userID, "is_read": false}).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("NotificationRepo - CountUnread - ToSql: %w", err)
	}

	var unread int
	if err := r.Pool.QueryRow(ctx, sql, args...).Scan(&unread); err != nil {
		return 0, fmt.Errorf("NotificationRepo - CountUnread - QueryRow: %w", err)
	}

	return unread, nil
}
