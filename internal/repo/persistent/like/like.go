package like

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/pkg/postgres"
	redispkg "github.com/evrone/go-clean-template/pkg/redis"
	"github.com/jackc/pgx/v5"
)

type Repo struct {
	*postgres.Postgres
	Redis *redispkg.Redis
}

func New(pg *postgres.Postgres, rdb *redispkg.Redis) *Repo {
	return &Repo{
		Postgres: pg,
		Redis:    rdb,
	}
}

// ToggleLike toggles like status for a video and updates Redis counter
func (r *Repo) ToggleLike(ctx context.Context, l *entity.VideoLike) (bool, int64, error) {
	// Check if already liked
	sql, args, err := r.Builder.
		Select("id").
		From("likes").
		Where(squirrel.Eq{"video_id": l.VideoID, "user_id": l.UserID}).
		ToSql()
	if err != nil {
		return false, 0, fmt.Errorf("LikeRepo - ToggleLike - ToSql: %w", err)
	}

	var existingID string
	err = r.Pool.QueryRow(ctx, sql, args...).Scan(&existingID)
	liked := false

	if err != nil {
		// Insert new like
		insSql, insArgs, _ := r.Builder.
			Insert("likes").
			Columns("id", "video_id", "user_id", "created_at").
			Values(l.ID, l.VideoID, l.UserID, time.Now().UTC()).
			ToSql()
		_, _ = r.Pool.Exec(ctx, insSql, insArgs...)
		liked = true
	} else {
		// Delete existing like
		delSql, delArgs, _ := r.Builder.
			Delete("likes").
			Where(squirrel.Eq{"id": existingID}).
			ToSql()
		_, _ = r.Pool.Exec(ctx, delSql, delArgs...)
		liked = false
	}

	// Update Redis counter
	var totalLikes int64
	if r.Redis != nil && r.Redis.Client != nil {
		key := fmt.Sprintf("video_likes:%s", l.VideoID)
		if liked {
			totalLikes, _ = r.Redis.Client.Incr(ctx, key).Result()
		} else {
			totalLikes, _ = r.Redis.Client.Decr(ctx, key).Result()
		}
	} else {
		// Fallback query from Postgres
		countSql, countArgs, _ := r.Builder.
			Select("COUNT(*)").
			From("likes").
			Where(squirrel.Eq{"video_id": l.VideoID}).
			ToSql()
		_ = r.Pool.QueryRow(ctx, countSql, countArgs...).Scan(&totalLikes)
	}

	if totalLikes < 0 {
		totalLikes = 0
	}

	return liked, totalLikes, nil
}

// IncrementHeart increases live stream heart counter in Redis
func (r *Repo) IncrementHeart(ctx context.Context, streamID string) (int64, error) {
	if r.Redis != nil && r.Redis.Client != nil {
		key := fmt.Sprintf("stream_hearts:%s", streamID)
		return r.Redis.Client.Incr(ctx, key).Result()
	}
	return 1, nil
}

// GetLikeCount returns the durable like count from Postgres — unlike the
// Redis counter ToggleLike maintains for fast increments/decrements, this is
// never reset by a Redis flush, so it's the right source for a fresh page
// load (GET /videos/:id) rather than the ephemeral cache.
func (r *Repo) GetLikeCount(ctx context.Context, videoID string) (int64, error) {
	sql, args, err := r.Builder.
		Select("COUNT(*)").
		From("likes").
		Where(squirrel.Eq{"video_id": videoID}).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("LikeRepo - GetLikeCount - ToSql: %w", err)
	}

	var count int64
	if err := r.Pool.QueryRow(ctx, sql, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("LikeRepo - GetLikeCount - QueryRow: %w", err)
	}
	return count, nil
}

// IsLikedByUser reports whether userID currently has an active like on
// videoID. userID may be empty (anonymous caller) — in that case this
// always returns false rather than erroring.
func (r *Repo) IsLikedByUser(ctx context.Context, videoID, userID string) (bool, error) {
	if userID == "" {
		return false, nil
	}

	sql, args, err := r.Builder.
		Select("1").
		From("likes").
		Where(squirrel.Eq{"video_id": videoID, "user_id": userID}).
		ToSql()
	if err != nil {
		return false, fmt.Errorf("LikeRepo - IsLikedByUser - ToSql: %w", err)
	}

	var exists int
	err = r.Pool.QueryRow(ctx, sql, args...).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("LikeRepo - IsLikedByUser - QueryRow: %w", err)
	}
	return true, nil
}
