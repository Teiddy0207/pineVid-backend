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

// ToggleLike flips the user's like on a video and returns the new liked
// state plus the total recounted from Postgres. The total deliberately isn't
// a Redis INCR/DECR counter: that counter was never seeded from Postgres, so
// it drifted from the GET /videos/:id count (e.g. 50 seeded likes → "1").
func (r *Repo) ToggleLike(ctx context.Context, l *entity.VideoLike) (bool, int64, error) {
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
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, 0, fmt.Errorf("LikeRepo - ToggleLike - QueryRow: %w", err)
	}

	liked := errors.Is(err, pgx.ErrNoRows)
	if liked {
		insSql, insArgs, _ := r.Builder.
			Insert("likes").
			Columns("id", "video_id", "user_id", "created_at").
			Values(l.ID, l.VideoID, l.UserID, time.Now().UTC()).
			ToSql()
		if _, err := r.Pool.Exec(ctx, insSql, insArgs...); err != nil {
			return false, 0, fmt.Errorf("LikeRepo - ToggleLike - Insert: %w", err)
		}
	} else {
		delSql, delArgs, _ := r.Builder.
			Delete("likes").
			Where(squirrel.Eq{"id": existingID}).
			ToSql()
		if _, err := r.Pool.Exec(ctx, delSql, delArgs...); err != nil {
			return false, 0, fmt.Errorf("LikeRepo - ToggleLike - Delete: %w", err)
		}
	}

	total, err := r.GetLikeCount(ctx, l.VideoID)
	if err != nil {
		return liked, 0, err
	}
	return liked, total, nil
}

// IncrementHeart increases live stream heart counter in Redis
func (r *Repo) IncrementHeart(ctx context.Context, streamID string) (int64, error) {
	if r.Redis != nil && r.Redis.Client != nil {
		key := fmt.Sprintf("stream_hearts:%s", streamID)
		return r.Redis.Client.Incr(ctx, key).Result()
	}
	return 1, nil
}

// GetLikeCount returns the durable like count for one video from Postgres.
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

// GetLikeInfo returns, in one query, the like count of every video in
// videoIDs and whether userID has liked each one — the batch counterpart of
// GetLikeCount/IsLikedByUser for list endpoints (reels feed), so a page of N
// videos costs one round-trip instead of 2N. Videos with no likes are simply
// absent from both maps; an empty userID never matches, so liked is all false.
func (r *Repo) GetLikeInfo(ctx context.Context, videoIDs []string, userID string) (map[string]int64, map[string]bool, error) {
	counts := make(map[string]int64, len(videoIDs))
	liked := make(map[string]bool, len(videoIDs))
	if len(videoIDs) == 0 {
		return counts, liked, nil
	}

	sql, args, err := r.Builder.
		Select("video_id", "COUNT(*)").
		Column(squirrel.Expr("BOOL_OR(user_id = ?)", userID)).
		From("likes").
		Where(squirrel.Eq{"video_id": videoIDs}).
		GroupBy("video_id").
		ToSql()
	if err != nil {
		return nil, nil, fmt.Errorf("LikeRepo - GetLikeInfo - ToSql: %w", err)
	}

	rows, err := r.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("LikeRepo - GetLikeInfo - Query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			videoID string
			count   int64
			byUser  bool
		)
		if err := rows.Scan(&videoID, &count, &byUser); err != nil {
			return nil, nil, fmt.Errorf("LikeRepo - GetLikeInfo - Scan: %w", err)
		}
		counts[videoID] = count
		liked[videoID] = byUser
	}
	return counts, liked, rows.Err()
}
