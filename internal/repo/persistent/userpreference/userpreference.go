package userpreference

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/pkg/postgres"
)

type Repo struct {
	*postgres.Postgres
}

func New(pg *postgres.Postgres) repo.UserPreferenceRepo {
	return &Repo{pg}
}

// SetCategories atomically replaces userID's preferred categories with
// categories — re-submitting preferences (e.g. from Settings later) fully
// replaces the previous set rather than appending to it.
func (r *Repo) SetCategories(ctx context.Context, userID string, categories []string) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("UserPreferenceRepo - SetCategories - Begin: %w", err)
	}
	defer tx.Rollback(ctx)

	delSQL, delArgs, err := r.Builder.Delete("user_category_preferences").Where(sq.Eq{"user_id": userID}).ToSql()
	if err != nil {
		return fmt.Errorf("UserPreferenceRepo - SetCategories - delete ToSql: %w", err)
	}
	if _, err := tx.Exec(ctx, delSQL, delArgs...); err != nil {
		return fmt.Errorf("UserPreferenceRepo - SetCategories - delete Exec: %w", err)
	}

	for _, category := range categories {
		insSQL, insArgs, err := r.Builder.
			Insert("user_category_preferences").
			Columns("user_id", "category").
			Values(userID, category).
			ToSql()
		if err != nil {
			return fmt.Errorf("UserPreferenceRepo - SetCategories - insert ToSql: %w", err)
		}
		if _, err := tx.Exec(ctx, insSQL, insArgs...); err != nil {
			return fmt.Errorf("UserPreferenceRepo - SetCategories - insert Exec: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("UserPreferenceRepo - SetCategories - Commit: %w", err)
	}
	return nil
}

func (r *Repo) GetCategories(ctx context.Context, userID string) ([]string, error) {
	sql, args, err := r.Builder.
		Select("category").
		From("user_category_preferences").
		Where(sq.Eq{"user_id": userID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("UserPreferenceRepo - GetCategories - ToSql: %w", err)
	}

	rows, err := r.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("UserPreferenceRepo - GetCategories - Query: %w", err)
	}
	defer rows.Close()

	categories := make([]string, 0)
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, fmt.Errorf("UserPreferenceRepo - GetCategories - Scan: %w", err)
		}
		categories = append(categories, c)
	}

	return categories, nil
}
