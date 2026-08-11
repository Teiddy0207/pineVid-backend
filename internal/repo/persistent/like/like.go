package like

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/pkg/postgres"
	redispkg "github.com/evrone/go-clean-template/pkg/redis"
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
