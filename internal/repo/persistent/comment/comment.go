package comment

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/pkg/postgres"
)

type Repo struct {
	*postgres.Postgres
}

func New(pg *postgres.Postgres) *Repo {
	return &Repo{Postgres: pg}
}

func (r *Repo) Store(ctx context.Context, c *entity.Comment) error {
	sql, args, err := r.Builder.
		Insert("comments").
		Columns("id", "video_id", "user_id", "user_name", "user_avatar", "content", "created_at").
		Values(c.ID, c.VideoID, c.UserID, c.UserName, c.UserAvatar, c.Content, time.Now().UTC()).
		ToSql()
	if err != nil {
		return fmt.Errorf("CommentRepo - Store - ToSql: %w", err)
	}

	_, err = r.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("CommentRepo - Store - Exec: %w", err)
	}

	return nil
}

func (r *Repo) ListByVideoID(ctx context.Context, videoID string, limit, offset uint64) ([]entity.Comment, int, error) {
	countSql, countArgs, err := r.Builder.
		Select("COUNT(*)").
		From("comments").
		Where(squirrel.Eq{"video_id": videoID}).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("CommentRepo - ListByVideoID - countSql: %w", err)
	}

	var total int
	if err := r.Pool.QueryRow(ctx, countSql, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("CommentRepo - ListByVideoID - countQuery: %w", err)
	}

	sql, args, err := r.Builder.
		Select("id", "video_id", "user_id", "user_name", "user_avatar", "content", "created_at").
		From("comments").
		Where(squirrel.Eq{"video_id": videoID}).
		OrderBy("created_at DESC").
		Limit(limit).
		Offset(offset).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("CommentRepo - ListByVideoID - ToSql: %w", err)
	}

	rows, err := r.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("CommentRepo - ListByVideoID - Query: %w", err)
	}
	defer rows.Close()

	comments := make([]entity.Comment, 0)
	for rows.Next() {
		var c entity.Comment
		if err := rows.Scan(&c.ID, &c.VideoID, &c.UserID, &c.UserName, &c.UserAvatar, &c.Content, &c.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("CommentRepo - ListByVideoID - Scan: %w", err)
		}
		comments = append(comments, c)
	}

	return comments, total, nil
}
