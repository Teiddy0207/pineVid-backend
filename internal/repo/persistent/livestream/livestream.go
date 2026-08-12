package livestream

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

func New(pg *postgres.Postgres) repo.LivestreamRepo {
	return &Repo{pg}
}

func (r *Repo) Store(ctx context.Context, ls *entity.Livestream) error {
	sql, args, err := r.Builder.
		Insert("livestreams").
		Columns("id", "user_id", "stream_key", "title", "category", "is_live", "hls_url", "viewers_count", "created_at", "updated_at").
		Values(ls.ID, ls.UserID, ls.StreamKey, ls.Title, ls.Category, ls.IsLive, ls.HLSUrl, ls.ViewersCount, ls.CreatedAt, ls.UpdatedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("LivestreamRepo - Store - r.Builder: %w", err)
	}

	_, err = r.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("LivestreamRepo - Store - Exec: %w", err)
	}

	return nil
}

var livestreamSelectColumns = []string{
	"ls.id", "ls.user_id", "COALESCE(u.username, u.email, 'Streamer')", "COALESCE(u.avatar_url, '')",
	"ls.stream_key", "ls.title", "ls.category", "ls.is_live", "ls.hls_url", "ls.viewers_count",
	"ls.started_at", "ls.ended_at", "ls.created_at", "ls.updated_at",
}

func scanLivestream(row pgx.Row, ls *entity.Livestream) error {
	return row.Scan(&ls.ID, &ls.UserID, &ls.UserName, &ls.UserAvatar, &ls.StreamKey, &ls.Title, &ls.Category, &ls.IsLive, &ls.HLSUrl, &ls.ViewersCount, &ls.StartedAt, &ls.EndedAt, &ls.CreatedAt, &ls.UpdatedAt)
}

func (r *Repo) GetByID(ctx context.Context, id string) (entity.Livestream, error) {
	sql, args, err := r.Builder.
		Select(livestreamSelectColumns...).
		From("livestreams ls").
		LeftJoin("users u ON ls.user_id::text = u.id::text").
		Where(sq.Eq{"ls.id": id}).
		ToSql()
	if err != nil {
		return entity.Livestream{}, fmt.Errorf("LivestreamRepo - GetByID - r.Builder: %w", err)
	}

	var ls entity.Livestream
	if err := scanLivestream(r.Pool.QueryRow(ctx, sql, args...), &ls); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Livestream{}, entity.ErrLivestreamNotFound
		}
		return entity.Livestream{}, fmt.Errorf("LivestreamRepo - GetByID - QueryRow: %w", err)
	}

	return ls, nil
}

func (r *Repo) GetByUserID(ctx context.Context, userID string) (entity.Livestream, error) {
	sql, args, err := r.Builder.
		Select(livestreamSelectColumns...).
		From("livestreams ls").
		LeftJoin("users u ON ls.user_id::text = u.id::text").
		Where(sq.Eq{"ls.user_id": userID}).
		ToSql()
	if err != nil {
		return entity.Livestream{}, fmt.Errorf("LivestreamRepo - GetByUserID - r.Builder: %w", err)
	}

	var ls entity.Livestream
	if err := scanLivestream(r.Pool.QueryRow(ctx, sql, args...), &ls); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Livestream{}, entity.ErrLivestreamNotFound
		}
		return entity.Livestream{}, fmt.Errorf("LivestreamRepo - GetByUserID - QueryRow: %w", err)
	}

	return ls, nil
}

func (r *Repo) GetByStreamKey(ctx context.Context, streamKey string) (entity.Livestream, error) {
	sql, args, err := r.Builder.
		Select(livestreamSelectColumns...).
		From("livestreams ls").
		LeftJoin("users u ON ls.user_id::text = u.id::text").
		Where(sq.Eq{"ls.stream_key": streamKey}).
		ToSql()
	if err != nil {
		return entity.Livestream{}, fmt.Errorf("LivestreamRepo - GetByStreamKey - r.Builder: %w", err)
	}

	var ls entity.Livestream
	if err := scanLivestream(r.Pool.QueryRow(ctx, sql, args...), &ls); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Livestream{}, entity.ErrInvalidStreamKey
		}
		return entity.Livestream{}, fmt.Errorf("LivestreamRepo - GetByStreamKey - QueryRow: %w", err)
	}

	return ls, nil
}

func (r *Repo) ListActive(ctx context.Context, category string, limit, offset int) ([]entity.Livestream, int, error) {
	countBuilder := r.Builder.Select("COUNT(*)").From("livestreams").Where(sq.Eq{"is_live": true})
	if category != "" {
		countBuilder = countBuilder.Where(sq.Eq{"category": category})
	}

	countSQL, countArgs, err := countBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("LivestreamRepo - ListActive - countBuilder: %w", err)
	}

	var total int
	err = r.Pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("LivestreamRepo - ListActive - count query: %w", err)
	}

	dataBuilder := r.Builder.
		Select(livestreamSelectColumns...).
		From("livestreams ls").
		LeftJoin("users u ON ls.user_id::text = u.id::text").
		Where(sq.Eq{"ls.is_live": true}).
		OrderBy("ls.viewers_count DESC").
		Limit(uint64(limit)).
		Offset(uint64(offset))

	if category != "" {
		dataBuilder = dataBuilder.Where(sq.Eq{"ls.category": category})
	}

	dataSQL, dataArgs, err := dataBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("LivestreamRepo - ListActive - dataBuilder: %w", err)
	}

	rows, err := r.Pool.Query(ctx, dataSQL, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("LivestreamRepo - ListActive - Query: %w", err)
	}
	defer rows.Close()

	streams := make([]entity.Livestream, 0, limit)
	for rows.Next() {
		var ls entity.Livestream
		if err := scanLivestream(rows, &ls); err != nil {
			return nil, 0, fmt.Errorf("LivestreamRepo - ListActive - rows.Scan: %w", err)
		}
		streams = append(streams, ls)
	}

	return streams, total, nil
}

func (r *Repo) Update(ctx context.Context, ls *entity.Livestream) error {
	sql, args, err := r.Builder.
		Update("livestreams").
		Set("stream_key", ls.StreamKey).
		Set("title", ls.Title).
		Set("category", ls.Category).
		Set("is_live", ls.IsLive).
		Set("hls_url", ls.HLSUrl).
		Set("viewers_count", ls.ViewersCount).
		Set("started_at", ls.StartedAt).
		Set("ended_at", ls.EndedAt).
		Set("updated_at", ls.UpdatedAt).
		Where(sq.Eq{"id": ls.ID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("LivestreamRepo - Update - r.Builder: %w", err)
	}

	_, err = r.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("LivestreamRepo - Update - Exec: %w", err)
	}

	return nil
}
