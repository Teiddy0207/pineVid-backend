// Package user implements the Postgres-backed User repository.
package user

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/pkg/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Repo -.
type Repo struct {
	*postgres.Postgres
}

// New returns a User repository instrumented with OpenTelemetry tracing spans.
func New(pg *postgres.Postgres) repo.UserRepo {
	return newTraced(&Repo{pg})
}

// Store -.
func (r *Repo) Store(ctx context.Context, user *entity.User) error {
	sql, args, err := r.Builder.
		Insert("users").
		Columns("id, username, email, password_hash, role, created_at, updated_at").
		Values(user.ID, user.Username, user.Email, user.PasswordHash, user.Role, user.CreatedAt, user.UpdatedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("UserRepo - Store - r.Builder: %w", err)
	}

	_, err = r.Pool.Exec(ctx, sql, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return entity.ErrUserAlreadyExists
		}

		return fmt.Errorf("UserRepo - Store - r.Pool.Exec: %w", err)
	}

	return nil
}

// GetByID -.
func (r *Repo) GetByID(ctx context.Context, id string) (entity.User, error) {
	return r.getUser(ctx, "id", id)
}

// GetByEmail -.
func (r *Repo) GetByEmail(ctx context.Context, email string) (entity.User, error) {
	return r.getUser(ctx, "email", email)
}

// GetByUsername -.
func (r *Repo) GetByUsername(ctx context.Context, username string) (entity.User, error) {
	return r.getUser(ctx, "username", username)
}

func (r *Repo) getUser(ctx context.Context, column, value string) (entity.User, error) {
	sql, args, err := r.Builder.
		Select("id, username, email, COALESCE(avatar_url, ''), password_hash, role, is_banned, created_at, updated_at").
		From("users").
		Where(sq.Eq{column: value}).
		ToSql()
	if err != nil {
		return entity.User{}, fmt.Errorf("UserRepo - getUser - r.Builder: %w", err)
	}

	var user entity.User

	err = r.Pool.QueryRow(ctx, sql, args...).
		Scan(&user.ID, &user.Username, &user.Email, &user.Avatar, &user.PasswordHash, &user.Role, &user.IsBanned, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.User{}, entity.ErrUserNotFound
		}

		return entity.User{}, fmt.Errorf("UserRepo - getUser - r.Pool.QueryRow: %w", err)
	}

	return user, nil
}

// List returns a page of users ordered by most recently created first.
func (r *Repo) List(ctx context.Context, page, limit int) ([]entity.User, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	var total int
	countSQL, _, err := r.Builder.Select("COUNT(*)").From("users").ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("UserRepo - List - countBuilder: %w", err)
	}
	if err := r.Pool.QueryRow(ctx, countSQL).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("UserRepo - List - count query: %w", err)
	}

	sql, args, err := r.Builder.
		Select("id, username, email, COALESCE(avatar_url, ''), password_hash, role, is_banned, created_at, updated_at").
		From("users").
		OrderBy("created_at DESC").
		Limit(uint64(limit)).
		Offset(uint64(offset)).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("UserRepo - List - r.Builder: %w", err)
	}

	rows, err := r.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("UserRepo - List - r.Pool.Query: %w", err)
	}
	defer rows.Close()

	users := make([]entity.User, 0, limit)
	for rows.Next() {
		var usr entity.User
		if err := rows.Scan(&usr.ID, &usr.Username, &usr.Email, &usr.Avatar, &usr.PasswordHash, &usr.Role, &usr.IsBanned, &usr.CreatedAt, &usr.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("UserRepo - List - rows.Scan: %w", err)
		}
		users = append(users, usr)
	}

	return users, total, nil
}

// Update persists username, email, and avatar_url changes for the given user.
func (r *Repo) Update(ctx context.Context, user *entity.User) error {
	sql, args, err := r.Builder.
		Update("users").
		Set("username", user.Username).
		Set("email", user.Email).
		Set("avatar_url", user.Avatar).
		Set("is_banned", user.IsBanned).
		Set("updated_at", user.UpdatedAt).
		Where(sq.Eq{"id": user.ID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("UserRepo - Update - r.Builder: %w", err)
	}

	_, err = r.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("UserRepo - Update - r.Pool.Exec: %w", err)
	}

	return nil
}

