package refreshtoken

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/pkg/postgres"
	"github.com/jackc/pgx/v5"
)

type Repo struct {
	*postgres.Postgres
}

func New(pg *postgres.Postgres) repo.RefreshTokenRepo {
	return &Repo{pg}
}

func (r *Repo) Store(ctx context.Context, t *entity.RefreshToken) error {
	sql, args, err := r.Builder.
		Insert("refresh_tokens").
		Columns("id", "user_id", "token_hash", "expires_at", "created_at").
		Values(t.ID, t.UserID, t.TokenHash, t.ExpiresAt, t.CreatedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("RefreshTokenRepo - Store - ToSql: %w", err)
	}

	if _, err := r.Pool.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("RefreshTokenRepo - Store - Exec: %w", err)
	}
	return nil
}

func (r *Repo) GetByHash(ctx context.Context, tokenHash string) (entity.RefreshToken, error) {
	sql, args, err := r.Builder.
		Select("id", "user_id", "token_hash", "expires_at", "revoked_at", "replaced_by", "created_at").
		From("refresh_tokens").
		Where(sq.Eq{"token_hash": tokenHash}).
		ToSql()
	if err != nil {
		return entity.RefreshToken{}, fmt.Errorf("RefreshTokenRepo - GetByHash - ToSql: %w", err)
	}

	var t entity.RefreshToken
	err = r.Pool.QueryRow(ctx, sql, args...).Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.RevokedAt, &t.ReplacedBy, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.RefreshToken{}, entity.ErrRefreshTokenNotFound
		}
		return entity.RefreshToken{}, fmt.Errorf("RefreshTokenRepo - GetByHash - QueryRow: %w", err)
	}
	return t, nil
}

// Revoke marks a single token as used/revoked. replacedBy is the ID of the
// token issued in exchange for it ("" on logout). Only an unrevoked token is
// updated, so two concurrent refreshes with the same token can't both win.
func (r *Repo) Revoke(ctx context.Context, id, replacedBy string) (bool, error) {
	b := r.Builder.
		Update("refresh_tokens").
		Set("revoked_at", time.Now().UTC()).
		Where(sq.Eq{"id": id}).
		Where("revoked_at IS NULL")
	if replacedBy != "" {
		b = b.Set("replaced_by", replacedBy)
	}

	sql, args, err := b.ToSql()
	if err != nil {
		return false, fmt.Errorf("RefreshTokenRepo - Revoke - ToSql: %w", err)
	}

	tag, err := r.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return false, fmt.Errorf("RefreshTokenRepo - Revoke - Exec: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *Repo) RevokeAllForUser(ctx context.Context, userID string) error {
	sql, args, err := r.Builder.
		Update("refresh_tokens").
		Set("revoked_at", time.Now().UTC()).
		Where(sq.Eq{"user_id": userID}).
		Where("revoked_at IS NULL").
		ToSql()
	if err != nil {
		return fmt.Errorf("RefreshTokenRepo - RevokeAllForUser - ToSql: %w", err)
	}

	if _, err := r.Pool.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("RefreshTokenRepo - RevokeAllForUser - Exec: %w", err)
	}
	return nil
}
