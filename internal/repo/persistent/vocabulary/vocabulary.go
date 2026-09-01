package vocabulary

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/pkg/postgres"
)

type Repo struct {
	*postgres.Postgres
}

func New(pg *postgres.Postgres) *Repo {
	return &Repo{pg}
}

func (r *Repo) SaveWord(ctx context.Context, item *entity.Vocabulary) error {
	sql, args, err := r.Builder.
		Insert("user_vocabulary").
		Columns("id", "user_id", "word", "ipa", "part_of_speech", "meaning", "example", "video_id", "video_title", "created_at").
		Values(item.ID, item.UserID, item.Word, item.IPA, item.PartOfSpeech, item.Meaning, item.Example, item.VideoID, item.VideoTitle, item.CreatedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("VocabularyRepo - SaveWord - ToSql: %w", err)
	}

	_, err = r.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("VocabularyRepo - SaveWord - Exec: %w", err)
	}
	return nil
}

func (r *Repo) ListByUserID(ctx context.Context, userID string) ([]entity.Vocabulary, error) {
	sql, args, err := r.Builder.
		Select("id", "user_id", "word", "ipa", "part_of_speech", "meaning", "example", "video_id", "video_title", "created_at").
		From("user_vocabulary").
		Where(squirrel.Eq{"user_id": userID}).
		OrderBy("created_at DESC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("VocabularyRepo - ListByUserID - ToSql: %w", err)
	}

	rows, err := r.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("VocabularyRepo - ListByUserID - Query: %w", err)
	}
	defer rows.Close()

	var items []entity.Vocabulary
	for rows.Next() {
		var item entity.Vocabulary
		err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Word,
			&item.IPA,
			&item.PartOfSpeech,
			&item.Meaning,
			&item.Example,
			&item.VideoID,
			&item.VideoTitle,
			&item.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("VocabularyRepo - ListByUserID - Scan: %w", err)
		}
		items = append(items, item)
	}

	return items, nil
}

func (r *Repo) DeleteWord(ctx context.Context, id, userID string) error {
	sql, args, err := r.Builder.
		Delete("user_vocabulary").
		Where(squirrel.Eq{"id": id, "user_id": userID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("VocabularyRepo - DeleteWord - ToSql: %w", err)
	}

	_, err = r.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("VocabularyRepo - DeleteWord - Exec: %w", err)
	}
	return nil
}
