package commentlike

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

// ToggleLike flips the current user's like on a comment and returns the new
// liked state plus the up-to-date total — mirrors LikeRepo.ToggleLike, minus
// the Redis fast-path (comment like counts don't need it at this scale).
func (r *Repo) ToggleLike(ctx context.Context, commentID, userID string) (bool, int64, error) {
	sql, args, err := r.Builder.
		Select("id").
		From("comment_likes").
		Where(squirrel.Eq{"comment_id": commentID, "user_id": userID}).
		ToSql()
	if err != nil {
		return false, 0, fmt.Errorf("CommentLikeRepo - ToggleLike - ToSql: %w", err)
	}

	var existingID string
	err = r.Pool.QueryRow(ctx, sql, args...).Scan(&existingID)
	liked := false

	if err != nil {
		insSql, insArgs, _ := r.Builder.
			Insert("comment_likes").
			Columns("id", "comment_id", "user_id", "created_at").
			Values(uuid.New().String(), commentID, userID, time.Now().UTC()).
			ToSql()
		if _, err := r.Pool.Exec(ctx, insSql, insArgs...); err != nil {
			return false, 0, fmt.Errorf("CommentLikeRepo - ToggleLike - Insert: %w", err)
		}
		liked = true
	} else {
		delSql, delArgs, _ := r.Builder.
			Delete("comment_likes").
			Where(squirrel.Eq{"id": existingID}).
			ToSql()
		if _, err := r.Pool.Exec(ctx, delSql, delArgs...); err != nil {
			return false, 0, fmt.Errorf("CommentLikeRepo - ToggleLike - Delete: %w", err)
		}
		liked = false
	}

	total, err := r.GetLikeCount(ctx, commentID)
	if err != nil {
		return liked, 0, err
	}
	return liked, total, nil
}

func (r *Repo) GetLikeCount(ctx context.Context, commentID string) (int64, error) {
	sql, args, err := r.Builder.
		Select("COUNT(*)").
		From("comment_likes").
		Where(squirrel.Eq{"comment_id": commentID}).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("CommentLikeRepo - GetLikeCount - ToSql: %w", err)
	}

	var count int64
	if err := r.Pool.QueryRow(ctx, sql, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("CommentLikeRepo - GetLikeCount - QueryRow: %w", err)
	}
	return count, nil
}

func (r *Repo) IsLikedByUser(ctx context.Context, commentID, userID string) (bool, error) {
	if userID == "" {
		return false, nil
	}

	sql, args, err := r.Builder.
		Select("1").
		From("comment_likes").
		Where(squirrel.Eq{"comment_id": commentID, "user_id": userID}).
		ToSql()
	if err != nil {
		return false, fmt.Errorf("CommentLikeRepo - IsLikedByUser - ToSql: %w", err)
	}

	var exists int
	err = r.Pool.QueryRow(ctx, sql, args...).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("CommentLikeRepo - IsLikedByUser - QueryRow: %w", err)
	}
	return true, nil
}
