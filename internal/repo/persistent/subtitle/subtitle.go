package subtitle

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/pkg/postgres"
	"github.com/google/uuid"
)

type Repo struct {
	*postgres.Postgres
}

func New(pg *postgres.Postgres) repo.SubtitleRepo {
	return &Repo{pg}
}

// ReplaceCues atomically swaps out all cues for videoID with a fresh set —
// re-uploading a subtitle track fully replaces the previous one rather than
// appending to it.
func (r *Repo) ReplaceCues(ctx context.Context, videoID string, cues []entity.SubtitleCue) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("SubtitleRepo - ReplaceCues - Begin: %w", err)
	}
	defer tx.Rollback(ctx)

	delSQL, delArgs, err := r.Builder.Delete("video_subtitles").Where(sq.Eq{"video_id": videoID}).ToSql()
	if err != nil {
		return fmt.Errorf("SubtitleRepo - ReplaceCues - delete ToSql: %w", err)
	}
	if _, err := tx.Exec(ctx, delSQL, delArgs...); err != nil {
		return fmt.Errorf("SubtitleRepo - ReplaceCues - delete Exec: %w", err)
	}

	for i, cue := range cues {
		id := cue.ID
		if id == "" {
			id = uuid.New().String()
		}
		insSQL, insArgs, err := r.Builder.
			Insert("video_subtitles").
			Columns("id", "video_id", "seq", "start_sec", "end_sec", "text_en", "text_vi").
			Values(id, videoID, i, cue.StartSec, cue.EndSec, cue.TextEN, cue.TextVI).
			ToSql()
		if err != nil {
			return fmt.Errorf("SubtitleRepo - ReplaceCues - insert ToSql: %w", err)
		}
		if _, err := tx.Exec(ctx, insSQL, insArgs...); err != nil {
			return fmt.Errorf("SubtitleRepo - ReplaceCues - insert Exec: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("SubtitleRepo - ReplaceCues - Commit: %w", err)
	}
	return nil
}

func (r *Repo) GetByVideoID(ctx context.Context, videoID string) ([]entity.SubtitleCue, error) {
	sql, args, err := r.Builder.
		Select("id", "start_sec", "end_sec", "text_en", "text_vi").
		From("video_subtitles").
		Where(sq.Eq{"video_id": videoID}).
		OrderBy("seq ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("SubtitleRepo - GetByVideoID - ToSql: %w", err)
	}

	rows, err := r.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("SubtitleRepo - GetByVideoID - Query: %w", err)
	}
	defer rows.Close()

	cues := make([]entity.SubtitleCue, 0)
	for rows.Next() {
		var c entity.SubtitleCue
		if err := rows.Scan(&c.ID, &c.StartSec, &c.EndSec, &c.TextEN, &c.TextVI); err != nil {
			return nil, fmt.Errorf("SubtitleRepo - GetByVideoID - Scan: %w", err)
		}
		cues = append(cues, c)
	}

	return cues, nil
}
