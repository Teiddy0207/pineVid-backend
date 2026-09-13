package post

import (
	"context"
	"errors"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/pkg/postgres"
	"github.com/jackc/pgx/v5"
)

type Repo struct {
	*postgres.Postgres
}

func New(pg *postgres.Postgres) *Repo {
	return &Repo{Postgres: pg}
}

func (r *Repo) Store(ctx context.Context, p *entity.Post) error {
	sql, args, err := r.Builder.
		Insert("posts").
		Columns("id", "user_id", "user_name", "user_avatar", "content", "image_url", "created_at", "updated_at").
		Values(p.ID, p.UserID, p.UserName, p.UserAvatar, p.Content, p.ImageURL, p.CreatedAt, p.UpdatedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("PostRepo - Store - ToSql: %w", err)
	}

	_, err = r.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("PostRepo - Store - Exec: %w", err)
	}

	return nil
}

func (r *Repo) GetByID(ctx context.Context, id string) (entity.Post, error) {
	sql, args, err := r.Builder.
		Select("id", "user_id", "user_name", "user_avatar", "content", "image_url", "created_at", "updated_at").
		From("posts").
		Where(squirrel.Eq{"id": id}).
		ToSql()
	if err != nil {
		return entity.Post{}, fmt.Errorf("PostRepo - GetByID - ToSql: %w", err)
	}

	var p entity.Post
	if err := r.Pool.QueryRow(ctx, sql, args...).
		Scan(&p.ID, &p.UserID, &p.UserName, &p.UserAvatar, &p.Content, &p.ImageURL, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Post{}, entity.ErrPostNotFound
		}
		return entity.Post{}, fmt.Errorf("PostRepo - GetByID - Scan: %w", err)
	}

	return p, nil
}

// ListByUser returns one user's posts newest-first — the content half of
// their public wall (see channel-detail-view.tsx on the frontend, which
// interleaves this with that same user's videos).
func (r *Repo) ListByUser(ctx context.Context, userID string, limit, offset uint64) ([]entity.Post, int, error) {
	countSql, countArgs, err := r.Builder.
		Select("COUNT(*)").
		From("posts").
		Where(squirrel.Eq{"user_id": userID}).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("PostRepo - ListByUser - countSql: %w", err)
	}

	var total int
	if err := r.Pool.QueryRow(ctx, countSql, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("PostRepo - ListByUser - countQuery: %w", err)
	}

	sql, args, err := r.Builder.
		Select("id", "user_id", "user_name", "user_avatar", "content", "image_url", "created_at", "updated_at").
		From("posts").
		Where(squirrel.Eq{"user_id": userID}).
		OrderBy("created_at DESC").
		Limit(limit).
		Offset(offset).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("PostRepo - ListByUser - ToSql: %w", err)
	}

	rows, err := r.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("PostRepo - ListByUser - Query: %w", err)
	}
	defer rows.Close()

	posts := make([]entity.Post, 0)
	for rows.Next() {
		var p entity.Post
		if err := rows.Scan(&p.ID, &p.UserID, &p.UserName, &p.UserAvatar, &p.Content, &p.ImageURL, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("PostRepo - ListByUser - Scan: %w", err)
		}
		posts = append(posts, p)
	}

	return posts, total, nil
}

func (r *Repo) Delete(ctx context.Context, id string) error {
	sql, args, err := r.Builder.
		Delete("posts").
		Where(squirrel.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("PostRepo - Delete - ToSql: %w", err)
	}

	_, err = r.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("PostRepo - Delete - Exec: %w", err)
	}

	return nil
}
