package video

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

func New(pg *postgres.Postgres) repo.VideoRepo {
	return &Repo{pg}
}

func (r *Repo) Store(ctx context.Context, v *entity.Video) error {
	sql, args, err := r.Builder.
		Insert("videos").
		Columns("id", "user_id", "title", "description", "category", "status", "visibility", "raw_s3_key", "hls_url", "thumbnail_url", "duration", "views", "created_at", "updated_at").
		Values(v.ID, v.UserID, v.Title, v.Description, v.Category, v.Status, v.Visibility, v.RawS3Key, v.HLSUrl, v.ThumbnailUrl, v.Duration, v.Views, v.CreatedAt, v.UpdatedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("VideoRepo - Store - r.Builder: %w", err)
	}

	_, err = r.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("VideoRepo - Store - r.Pool.Exec: %w", err)
	}

	return nil
}

func (r *Repo) GetByID(ctx context.Context, id string) (entity.Video, error) {
	sql, args, err := r.Builder.
		Select("id", "user_id", "title", "description", "category", "status", "visibility", "raw_s3_key", "hls_url", "thumbnail_url", "duration", "views", "created_at", "updated_at").
		From("videos").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return entity.Video{}, fmt.Errorf("VideoRepo - GetByID - r.Builder: %w", err)
	}

	var v entity.Video
	err = r.Pool.QueryRow(ctx, sql, args...).
		Scan(&v.ID, &v.UserID, &v.Title, &v.Description, &v.Category, &v.Status, &v.Visibility, &v.RawS3Key, &v.HLSUrl, &v.ThumbnailUrl, &v.Duration, &v.Views, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Video{}, entity.ErrVideoNotFound
		}
		return entity.Video{}, fmt.Errorf("VideoRepo - GetByID - QueryRow: %w", err)
	}

	return v, nil
}

func (r *Repo) List(ctx context.Context, filter repo.VideoFilter) ([]entity.Video, int, error) {
	countBuilder := r.Builder.Select("COUNT(*)").From("videos")

	if filter.UserID != "" {
		countBuilder = countBuilder.Where(sq.Eq{"user_id": filter.UserID})
	}
	if filter.Category != "" {
		countBuilder = countBuilder.Where(sq.Eq{"category": filter.Category})
	}
	if filter.Status != nil {
		countBuilder = countBuilder.Where(sq.Eq{"status": *filter.Status})
	}
	if filter.Visibility != nil {
		countBuilder = countBuilder.Where(sq.Eq{"visibility": *filter.Visibility})
	}

	countSQL, countArgs, err := countBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("VideoRepo - List - countBuilder: %w", err)
	}

	var total int
	err = r.Pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("VideoRepo - List - count query: %w", err)
	}

	dataBuilder := r.Builder.
		Select("id", "user_id", "title", "description", "category", "status", "visibility", "raw_s3_key", "hls_url", "thumbnail_url", "duration", "views", "created_at", "updated_at").
		From("videos").
		OrderBy("created_at DESC").
		Limit(filter.Limit).
		Offset(filter.Offset)

	if filter.UserID != "" {
		dataBuilder = dataBuilder.Where(sq.Eq{"user_id": filter.UserID})
	}
	if filter.Category != "" {
		dataBuilder = dataBuilder.Where(sq.Eq{"category": filter.Category})
	}
	if filter.Status != nil {
		dataBuilder = dataBuilder.Where(sq.Eq{"status": *filter.Status})
	}
	if filter.Visibility != nil {
		dataBuilder = dataBuilder.Where(sq.Eq{"visibility": *filter.Visibility})
	}

	dataSQL, dataArgs, err := dataBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("VideoRepo - List - dataBuilder: %w", err)
	}

	rows, err := r.Pool.Query(ctx, dataSQL, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("VideoRepo - List - Query: %w", err)
	}
	defer rows.Close()

	videos := make([]entity.Video, 0, filter.Limit)
	for rows.Next() {
		var v entity.Video
		err = rows.Scan(&v.ID, &v.UserID, &v.Title, &v.Description, &v.Category, &v.Status, &v.Visibility, &v.RawS3Key, &v.HLSUrl, &v.ThumbnailUrl, &v.Duration, &v.Views, &v.CreatedAt, &v.UpdatedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("VideoRepo - List - rows.Scan: %w", err)
		}
		videos = append(videos, v)
	}

	return videos, total, nil
}

func (r *Repo) Update(ctx context.Context, v *entity.Video) error {
	sql, args, err := r.Builder.
		Update("videos").
		Set("title", v.Title).
		Set("description", v.Description).
		Set("category", v.Category).
		Set("status", v.Status).
		Set("visibility", v.Visibility).
		Set("hls_url", v.HLSUrl).
		Set("thumbnail_url", v.ThumbnailUrl).
		Set("duration", v.Duration).
		Set("views", v.Views).
		Set("updated_at", v.UpdatedAt).
		Where(sq.Eq{"id": v.ID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("VideoRepo - Update - r.Builder: %w", err)
	}

	_, err = r.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("VideoRepo - Update - Exec: %w", err)
	}

	return nil
}

func (r *Repo) Delete(ctx context.Context, id string) error {
	sql, args, err := r.Builder.
		Delete("videos").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("VideoRepo - Delete - r.Builder: %w", err)
	}

	_, err = r.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("VideoRepo - Delete - Exec: %w", err)
	}

	return nil
}
