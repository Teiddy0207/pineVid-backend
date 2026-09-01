package follow

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/pkg/postgres"
	"github.com/jackc/pgx/v5"
)

type Repo struct {
	*postgres.Postgres
}

func New(pg *postgres.Postgres) repo.FollowRepo {
	return &Repo{pg}
}

func (r *Repo) Follow(ctx context.Context, followerID, channelID string) error {
	sql, args, err := r.Builder.
		Insert("follows").
		Columns("follower_id", "channel_id", "created_at").
		Values(followerID, channelID, sq.Expr("CURRENT_TIMESTAMP")).
		Suffix("ON CONFLICT (follower_id, channel_id) DO NOTHING").
		ToSql()
	if err != nil {
		return fmt.Errorf("FollowRepo - Follow - r.Builder: %w", err)
	}

	if _, err := r.Pool.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("FollowRepo - Follow - Exec: %w", err)
	}
	return nil
}

func (r *Repo) Unfollow(ctx context.Context, followerID, channelID string) error {
	sql, args, err := r.Builder.
		Delete("follows").
		Where(sq.Eq{"follower_id": followerID, "channel_id": channelID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("FollowRepo - Unfollow - r.Builder: %w", err)
	}

	if _, err := r.Pool.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("FollowRepo - Unfollow - Exec: %w", err)
	}
	return nil
}

func (r *Repo) IsFollowing(ctx context.Context, followerID, channelID string) (bool, error) {
	sql, args, err := r.Builder.
		Select("1").
		From("follows").
		Where(sq.Eq{"follower_id": followerID, "channel_id": channelID}).
		ToSql()
	if err != nil {
		return false, fmt.Errorf("FollowRepo - IsFollowing - r.Builder: %w", err)
	}

	var exists int
	err = r.Pool.QueryRow(ctx, sql, args...).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("FollowRepo - IsFollowing - QueryRow: %w", err)
	}
	return true, nil
}

func (r *Repo) CountFollowers(ctx context.Context, channelID string) (int64, error) {
	sql, args, err := r.Builder.
		Select("COUNT(*)").
		From("follows").
		Where(sq.Eq{"channel_id": channelID}).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("FollowRepo - CountFollowers - r.Builder: %w", err)
	}

	var count int64
	if err := r.Pool.QueryRow(ctx, sql, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("FollowRepo - CountFollowers - QueryRow: %w", err)
	}
	return count, nil
}

func (r *Repo) ListFollowedChannels(ctx context.Context, followerID string, page, limit int) ([]entity.User, int, error) {
	offset := (page - 1) * limit

	countSQL, countArgs, err := r.Builder.
		Select("COUNT(*)").
		From("follows").
		Where(sq.Eq{"follower_id": followerID}).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("FollowRepo - ListFollowedChannels - countBuilder: %w", err)
	}

	var total int
	if err := r.Pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("FollowRepo - ListFollowedChannels - count query: %w", err)
	}

	dataSQL, dataArgs, err := r.Builder.
		Select("u.id", "u.username", "u.email", "COALESCE(u.avatar_url, '')", "u.password_hash", "u.is_banned", "u.created_at", "u.updated_at").
		From("follows f").
		Join("users u ON u.id::text = f.channel_id").
		Where(sq.Eq{"f.follower_id": followerID}).
		OrderBy("f.created_at DESC").
		Limit(uint64(limit)).
		Offset(uint64(offset)).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("FollowRepo - ListFollowedChannels - dataBuilder: %w", err)
	}

	rows, err := r.Pool.Query(ctx, dataSQL, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("FollowRepo - ListFollowedChannels - Query: %w", err)
	}
	defer rows.Close()

	channels := make([]entity.User, 0, limit)
	for rows.Next() {
		var u entity.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Avatar, &u.PasswordHash, &u.IsBanned, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("FollowRepo - ListFollowedChannels - rows.Scan: %w", err)
		}
		channels = append(channels, u)
	}

	return channels, total, nil
}

// ListFollowers returns the users who follow channelID (i.e. the reverse of
// ListFollowedChannels) — used to fan out notifications to a streamer's/
// uploader's followers.
func (r *Repo) ListFollowers(ctx context.Context, channelID string, limit int) ([]entity.User, error) {
	sql, args, err := r.Builder.
		Select("u.id", "u.username", "u.email", "COALESCE(u.avatar_url, '')", "u.password_hash", "u.is_banned", "u.created_at", "u.updated_at").
		From("follows f").
		Join("users u ON u.id::text = f.follower_id").
		Where(sq.Eq{"f.channel_id": channelID}).
		OrderBy("f.created_at DESC").
		Limit(uint64(limit)).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("FollowRepo - ListFollowers - r.Builder: %w", err)
	}

	rows, err := r.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("FollowRepo - ListFollowers - Query: %w", err)
	}
	defer rows.Close()

	followers := make([]entity.User, 0, limit)
	for rows.Next() {
		var u entity.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Avatar, &u.PasswordHash, &u.IsBanned, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("FollowRepo - ListFollowers - rows.Scan: %w", err)
		}
		followers = append(followers, u)
	}

	return followers, nil
}
