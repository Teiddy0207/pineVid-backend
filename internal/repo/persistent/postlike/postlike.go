package postlike

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/evrone/go-clean-template/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Repo struct {
	*postgres.Postgres
}

func New(pg *postgres.Postgres) *Repo {
	return &Repo{Postgres: pg}
}

// ToggleLike flips the current user's like on a post and returns the new
// liked state plus the up-to-date total — mirrors CommentLikeRepo.ToggleLike.
func (r *Repo) ToggleLike(ctx context.Context, postID, userID string) (bool, int64, error) {
	sql, args, err := r.Builder.
		Select("id").
		From("post_likes").
		Where(squirrel.Eq{"post_id": postID, "user_id": userID}).
		ToSql()
	if err != nil {
		return false, 0, fmt.Errorf("PostLikeRepo - ToggleLike - ToSql: %w", err)
	}

	var existingID string
	err = r.Pool.QueryRow(ctx, sql, args...).Scan(&existingID)
	liked := false

	if err != nil {
		insSql, insArgs, _ := r.Builder.
			Insert("post_likes").
			Columns("id", "post_id", "user_id", "created_at").
			Values(uuid.New().String(), postID, userID, time.Now().UTC()).
			ToSql()
		if _, err := r.Pool.Exec(ctx, insSql, insArgs...); err != nil {
			return false, 0, fmt.Errorf("PostLikeRepo - ToggleLike - Insert: %w", err)
		}
		liked = true
	} else {
		delSql, delArgs, _ := r.Builder.
			Delete("post_likes").
			Where(squirrel.Eq{"id": existingID}).
			ToSql()
		if _, err := r.Pool.Exec(ctx, delSql, delArgs...); err != nil {
			return false, 0, fmt.Errorf("PostLikeRepo - ToggleLike - Delete: %w", err)
		}
		liked = false
	}

	total, err := r.GetLikeCount(ctx, postID)
	if err != nil {
		return liked, 0, err
	}
	return liked, total, nil
}

func (r *Repo) GetLikeCount(ctx context.Context, postID string) (int64, error) {
	sql, args, err := r.Builder.
		Select("COUNT(*)").
		From("post_likes").
		Where(squirrel.Eq{"post_id": postID}).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("PostLikeRepo - GetLikeCount - ToSql: %w", err)
	}

	var count int64
	if err := r.Pool.QueryRow(ctx, sql, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("PostLikeRepo - GetLikeCount - QueryRow: %w", err)
	}
	return count, nil
}

func (r *Repo) IsLikedByUser(ctx context.Context, postID, userID string) (bool, error) {
	if userID == "" {
		return false, nil
	}

	sql, args, err := r.Builder.
		Select("1").
		From("post_likes").
		Where(squirrel.Eq{"post_id": postID, "user_id": userID}).
		ToSql()
	if err != nil {
		return false, fmt.Errorf("PostLikeRepo - IsLikedByUser - ToSql: %w", err)
	}

	var exists int
	err = r.Pool.QueryRow(ctx, sql, args...).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("PostLikeRepo - IsLikedByUser - QueryRow: %w", err)
	}
	return true, nil
}
