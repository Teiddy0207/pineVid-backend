package history

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/pkg/postgres"
)

type Repo struct {
	*postgres.Postgres
}

func New(pg *postgres.Postgres) repo.WatchHistoryRepo {
	return &Repo{pg}
}

// Upsert records/updates the user's watch progress for a video. One row per
// (user_id, video_id) — rewatching just bumps last_watched_at and watch_seconds.
func (r *Repo) Upsert(ctx context.Context, userID, videoID string, watchSeconds int) error {
	sql, args, err := r.Builder.
		Insert("watch_history").
		Columns("user_id", "video_id", "watch_seconds", "last_watched_at").
		Values(userID, videoID, watchSeconds, sq.Expr("CURRENT_TIMESTAMP")).
		Suffix("ON CONFLICT (user_id, video_id) DO UPDATE SET watch_seconds = GREATEST(watch_history.watch_seconds, EXCLUDED.watch_seconds), last_watched_at = EXCLUDED.last_watched_at").
		ToSql()
	if err != nil {
		return fmt.Errorf("WatchHistoryRepo - Upsert - r.Builder: %w", err)
	}

	if _, err := r.Pool.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("WatchHistoryRepo - Upsert - Exec: %w", err)
	}

	return nil
}

// ListByUser returns the videos a user has watched, most recently watched first,
// joined with the video + creator info (reusing entity.Video like VideoRepo.List).
func (r *Repo) ListByUser(ctx context.Context, userID string, limit, offset int) ([]entity.Video, int, error) {
	countSQL, countArgs, err := r.Builder.
		Select("COUNT(*)").
		From("watch_history").
		Where(sq.Eq{"user_id": userID}).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("WatchHistoryRepo - ListByUser - countBuilder: %w", err)
	}

	var total int
	if err := r.Pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("WatchHistoryRepo - ListByUser - count query: %w", err)
	}

	dataSQL, dataArgs, err := r.Builder.
		Select("v.id", "v.user_id", "COALESCE(u.username, u.email, 'Creator')", "COALESCE(u.avatar_url, '')", "v.title", "v.description", "v.category", "v.status", "v.visibility", "v.raw_s3_key", "v.hls_url", "v.thumbnail_url", "v.duration", "v.views", "v.created_at", "v.updated_at").
		From("watch_history wh").
		Join("videos v ON wh.video_id = v.id").
		LeftJoin("users u ON v.user_id::text = u.id::text").
		Where(sq.Eq{"wh.user_id": userID}).
		OrderBy("wh.last_watched_at DESC").
		Limit(uint64(limit)).
		Offset(uint64(offset)).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("WatchHistoryRepo - ListByUser - dataBuilder: %w", err)
	}

	rows, err := r.Pool.Query(ctx, dataSQL, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("WatchHistoryRepo - ListByUser - Query: %w", err)
	}
	defer rows.Close()

	videos := make([]entity.Video, 0, limit)
	for rows.Next() {
		var v entity.Video
		if err := rows.Scan(&v.ID, &v.UserID, &v.UserName, &v.UserAvatar, &v.Title, &v.Description, &v.Category, &v.Status, &v.Visibility, &v.RawS3Key, &v.HLSUrl, &v.ThumbnailUrl, &v.Duration, &v.Views, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("WatchHistoryRepo - ListByUser - rows.Scan: %w", err)
		}
		videos = append(videos, v)
	}

	return videos, total, nil
}
