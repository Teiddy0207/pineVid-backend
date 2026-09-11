// Package savedvideo implements the "Save to Watch Later" feature — a
// per-user bookmark list, structurally identical to likes (toggle table +
// unique(user_id, video_id)) but semantically separate from liking.
package savedvideo

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Repo struct {
	*postgres.Postgres
}

func New(pg *postgres.Postgres) repo.SavedVideoRepo {
	return &Repo{pg}
}

// ToggleSave adds or removes videoID from userID's saved list, returning the
// resulting saved state (true if now saved).
func (r *Repo) ToggleSave(ctx context.Context, userID, videoID string) (bool, error) {
	sql, args, err := r.Builder.
		Select("id").
		From("saved_videos").
		Where(sq.Eq{"video_id": videoID, "user_id": userID}).
		ToSql()
	if err != nil {
		return false, fmt.Errorf("SavedVideoRepo - ToggleSave - ToSql: %w", err)
	}

	var existingID string
	err = r.Pool.QueryRow(ctx, sql, args...).Scan(&existingID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return false, fmt.Errorf("SavedVideoRepo - ToggleSave - QueryRow: %w", err)
		}

		insSQL, insArgs, err := r.Builder.
			Insert("saved_videos").
			Columns("id", "video_id", "user_id", "created_at").
			Values(uuid.New().String(), videoID, userID, time.Now().UTC()).
			ToSql()
		if err != nil {
			return false, fmt.Errorf("SavedVideoRepo - ToggleSave - insert ToSql: %w", err)
		}
		if _, err := r.Pool.Exec(ctx, insSQL, insArgs...); err != nil {
			return false, fmt.Errorf("SavedVideoRepo - ToggleSave - insert Exec: %w", err)
		}
		return true, nil
	}

	delSQL, delArgs, err := r.Builder.Delete("saved_videos").Where(sq.Eq{"id": existingID}).ToSql()
	if err != nil {
		return false, fmt.Errorf("SavedVideoRepo - ToggleSave - delete ToSql: %w", err)
	}
	if _, err := r.Pool.Exec(ctx, delSQL, delArgs...); err != nil {
		return false, fmt.Errorf("SavedVideoRepo - ToggleSave - delete Exec: %w", err)
	}
	return false, nil
}

// IsSavedByUser reports whether userID currently has videoID saved. userID
// may be empty (anonymous caller), in which case this always returns false.
func (r *Repo) IsSavedByUser(ctx context.Context, videoID, userID string) (bool, error) {
	if userID == "" {
		return false, nil
	}

	sql, args, err := r.Builder.
		Select("1").
		From("saved_videos").
		Where(sq.Eq{"video_id": videoID, "user_id": userID}).
		ToSql()
	if err != nil {
		return false, fmt.Errorf("SavedVideoRepo - IsSavedByUser - ToSql: %w", err)
	}

	var exists int
	err = r.Pool.QueryRow(ctx, sql, args...).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("SavedVideoRepo - IsSavedByUser - QueryRow: %w", err)
	}
	return true, nil
}

// ListSavedVideos returns the videos a user has saved, most recently saved
// first, joined with video + creator info (mirrors WatchHistoryRepo.ListByUser).
func (r *Repo) ListSavedVideos(ctx context.Context, userID string, limit, offset int) ([]entity.Video, int, error) {
	countSQL, countArgs, err := r.Builder.
		Select("COUNT(*)").
		From("saved_videos").
		Where(sq.Eq{"user_id": userID}).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("SavedVideoRepo - ListSavedVideos - countBuilder: %w", err)
	}

	var total int
	if err := r.Pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("SavedVideoRepo - ListSavedVideos - count query: %w", err)
	}

	dataSQL, dataArgs, err := r.Builder.
		Select("v.id", "v.user_id", "COALESCE(u.username, u.email, 'Creator')", "COALESCE(u.avatar_url, '')", "v.title", "v.description", "v.category", "v.status", "v.visibility", "v.raw_s3_key", "v.hls_url", "v.thumbnail_url", "v.duration", "v.views", "v.created_at", "v.updated_at").
		From("saved_videos sv").
		Join("videos v ON sv.video_id = v.id").
		LeftJoin("users u ON v.user_id::text = u.id::text").
		Where(sq.Eq{"sv.user_id": userID}).
		OrderBy("sv.created_at DESC").
		Limit(uint64(limit)).
		Offset(uint64(offset)).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("SavedVideoRepo - ListSavedVideos - dataBuilder: %w", err)
	}

	rows, err := r.Pool.Query(ctx, dataSQL, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("SavedVideoRepo - ListSavedVideos - Query: %w", err)
	}
	defer rows.Close()

	videos := make([]entity.Video, 0, limit)
	for rows.Next() {
		var v entity.Video
		if err := rows.Scan(&v.ID, &v.UserID, &v.UserName, &v.UserAvatar, &v.Title, &v.Description, &v.Category, &v.Status, &v.Visibility, &v.RawS3Key, &v.HLSUrl, &v.ThumbnailUrl, &v.Duration, &v.Views, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("SavedVideoRepo - ListSavedVideos - rows.Scan: %w", err)
		}
		videos = append(videos, v)
	}

	return videos, total, nil
}
