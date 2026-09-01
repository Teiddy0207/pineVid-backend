package comment

import (
	"context"
	"errors"
	"fmt"
	"time"

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

func (r *Repo) Store(ctx context.Context, c *entity.Comment) error {
	sql, args, err := r.Builder.
		Insert("comments").
		Columns("id", "video_id", "user_id", "user_name", "user_avatar", "content", "parent_id", "created_at").
		Values(c.ID, c.VideoID, c.UserID, c.UserName, c.UserAvatar, c.Content, c.ParentID, time.Now().UTC()).
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

// GetByID is used to validate that a reply's parent_id points at a real
// comment before insert — a self-contained lookup rather than reusing
// ListByVideoID, which deliberately excludes replies from its results.
func (r *Repo) GetByID(ctx context.Context, id string) (entity.Comment, error) {
	sql, args, err := r.Builder.
		Select("id", "video_id", "user_id", "user_name", "user_avatar", "content", "parent_id", "created_at").
		From("comments").
		Where(squirrel.Eq{"id": id}).
		ToSql()
	if err != nil {
		return entity.Comment{}, fmt.Errorf("CommentRepo - GetByID - ToSql: %w", err)
	}

	var c entity.Comment
	if err := r.Pool.QueryRow(ctx, sql, args...).
		Scan(&c.ID, &c.VideoID, &c.UserID, &c.UserName, &c.UserAvatar, &c.Content, &c.ParentID, &c.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Comment{}, entity.ErrCommentNotFound
		}
		return entity.Comment{}, fmt.Errorf("CommentRepo - GetByID - Scan: %w", err)
	}

	return c, nil
}

// ListByVideoID returns top-level comments only — replies are fetched
// separately via ListRepliesByParentID so a video's main comment feed isn't
// diluted by nested replies.
func (r *Repo) ListByVideoID(ctx context.Context, videoID string, limit, offset uint64) ([]entity.Comment, int, error) {
	countSql, countArgs, err := r.Builder.
		Select("COUNT(*)").
		From("comments").
		Where(squirrel.Eq{"video_id": videoID, "parent_id": nil}).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("CommentRepo - ListByVideoID - countSql: %w", err)
	}

	var total int
	if err := r.Pool.QueryRow(ctx, countSql, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("CommentRepo - ListByVideoID - countQuery: %w", err)
	}

	sql, args, err := r.Builder.
		Select("id", "video_id", "user_id", "user_name", "user_avatar", "content", "parent_id", "created_at").
		From("comments").
		Where(squirrel.Eq{"video_id": videoID, "parent_id": nil}).
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
		if err := rows.Scan(&c.ID, &c.VideoID, &c.UserID, &c.UserName, &c.UserAvatar, &c.Content, &c.ParentID, &c.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("CommentRepo - ListByVideoID - Scan: %w", err)
		}
		comments = append(comments, c)
	}

	return comments, total, nil
}

// ListRepliesByParentID returns replies to a single top-level comment,
// oldest first (conversational order, unlike the newest-first main feed).
func (r *Repo) ListRepliesByParentID(ctx context.Context, parentID string, limit, offset uint64) ([]entity.Comment, int, error) {
	countSql, countArgs, err := r.Builder.
		Select("COUNT(*)").
		From("comments").
		Where(squirrel.Eq{"parent_id": parentID}).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("CommentRepo - ListRepliesByParentID - countSql: %w", err)
	}

	var total int
	if err := r.Pool.QueryRow(ctx, countSql, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("CommentRepo - ListRepliesByParentID - countQuery: %w", err)
	}

	sql, args, err := r.Builder.
		Select("id", "video_id", "user_id", "user_name", "user_avatar", "content", "parent_id", "created_at").
		From("comments").
		Where(squirrel.Eq{"parent_id": parentID}).
		OrderBy("created_at ASC").
		Limit(limit).
		Offset(offset).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("CommentRepo - ListRepliesByParentID - ToSql: %w", err)
	}

	rows, err := r.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("CommentRepo - ListRepliesByParentID - Query: %w", err)
	}
	defer rows.Close()

	replies := make([]entity.Comment, 0)
	for rows.Next() {
		var c entity.Comment
		if err := rows.Scan(&c.ID, &c.VideoID, &c.UserID, &c.UserName, &c.UserAvatar, &c.Content, &c.ParentID, &c.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("CommentRepo - ListRepliesByParentID - Scan: %w", err)
		}
		replies = append(replies, c)
	}

	return replies, total, nil
}
