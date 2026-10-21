package livesession

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

func New(pg *postgres.Postgres) repo.LiveSessionRepo {
	return &Repo{pg}
}

// The stats columns come from live_session_stats (written by live-agg); a
// session without any processed event yet reads as zeros.
var sessionColumns = []string{
	"id", "livestream_id", "title", "thumbnail_url", "category",
	"started_at", "ended_at", "COALESCE(replay_video_id, '')", "created_at",
	"COALESCE((SELECT st.hearts FROM live_session_stats st WHERE st.session_id = live_sessions.id), 0)",
	"COALESCE((SELECT st.chat_messages FROM live_session_stats st WHERE st.session_id = live_sessions.id), 0)",
	"COALESCE((SELECT st.unique_chatters FROM live_session_stats st WHERE st.session_id = live_sessions.id), 0)",
}

func scanSession(row pgx.Row, s *entity.LiveSession) error {
	return row.Scan(
		&s.ID, &s.LivestreamID, &s.Title, &s.ThumbnailURL, &s.Category,
		&s.StartedAt, &s.EndedAt, &s.ReplayVideoID, &s.CreatedAt,
		&s.Stats.Hearts, &s.Stats.ChatMessages, &s.Stats.UniqueChatters,
	)
}

func (r *Repo) Store(ctx context.Context, sess *entity.LiveSession) error {
	sql, args, err := r.Builder.
		Insert("live_sessions").
		Columns("id", "livestream_id", "title", "thumbnail_url", "category", "started_at", "created_at").
		Values(sess.ID, sess.LivestreamID, sess.Title, sess.ThumbnailURL, sess.Category, sess.StartedAt, sess.CreatedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("LiveSessionRepo - Store - Builder: %w", err)
	}

	if _, err := r.Pool.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("LiveSessionRepo - Store - Exec: %w", err)
	}
	return nil
}

func (r *Repo) GetByID(ctx context.Context, id string) (entity.LiveSession, error) {
	sql, args, err := r.Builder.
		Select(sessionColumns...).
		From("live_sessions").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return entity.LiveSession{}, fmt.Errorf("LiveSessionRepo - GetByID - Builder: %w", err)
	}

	var s entity.LiveSession
	if err := scanSession(r.Pool.QueryRow(ctx, sql, args...), &s); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.LiveSession{}, entity.ErrLiveSessionNotFound
		}
		return entity.LiveSession{}, fmt.Errorf("LiveSessionRepo - GetByID - QueryRow: %w", err)
	}
	return s, nil
}

func (r *Repo) GetCurrentByLivestreamID(ctx context.Context, livestreamID string) (entity.LiveSession, error) {
	sql, args, err := r.Builder.
		Select(sessionColumns...).
		From("live_sessions").
		Where(sq.Eq{"livestream_id": livestreamID}).
		Where("ended_at IS NULL").
		OrderBy("started_at DESC").
		Limit(1).
		ToSql()
	if err != nil {
		return entity.LiveSession{}, fmt.Errorf("LiveSessionRepo - GetCurrentByLivestreamID - Builder: %w", err)
	}

	var s entity.LiveSession
	if err := scanSession(r.Pool.QueryRow(ctx, sql, args...), &s); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.LiveSession{}, entity.ErrLiveSessionNotFound
		}
		return entity.LiveSession{}, fmt.Errorf("LiveSessionRepo - GetCurrentByLivestreamID - QueryRow: %w", err)
	}
	return s, nil
}

func (r *Repo) ListByLivestreamID(ctx context.Context, livestreamID string, limit, offset int) ([]entity.LiveSession, int, error) {
	countSQL, countArgs, err := r.Builder.
		Select("COUNT(*)").
		From("live_sessions").
		Where(sq.Eq{"livestream_id": livestreamID}).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("LiveSessionRepo - ListByLivestreamID - countBuilder: %w", err)
	}

	var total int
	if err := r.Pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("LiveSessionRepo - ListByLivestreamID - count: %w", err)
	}

	dataSQL, dataArgs, err := r.Builder.
		Select(sessionColumns...).
		From("live_sessions").
		Where(sq.Eq{"livestream_id": livestreamID}).
		OrderBy("started_at DESC").
		Limit(uint64(limit)).
		Offset(uint64(offset)).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("LiveSessionRepo - ListByLivestreamID - dataBuilder: %w", err)
	}

	rows, err := r.Pool.Query(ctx, dataSQL, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("LiveSessionRepo - ListByLivestreamID - Query: %w", err)
	}
	defer rows.Close()

	sessions := make([]entity.LiveSession, 0, limit)
	for rows.Next() {
		var s entity.LiveSession
		if err := scanSession(rows, &s); err != nil {
			return nil, 0, fmt.Errorf("LiveSessionRepo - ListByLivestreamID - Scan: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, total, nil
}

func (r *Repo) EndSession(ctx context.Context, id string, endedAt time.Time) error {
	sql, args, err := r.Builder.
		Update("live_sessions").
		Set("ended_at", endedAt).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("LiveSessionRepo - EndSession - Builder: %w", err)
	}

	if _, err := r.Pool.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("LiveSessionRepo - EndSession - Exec: %w", err)
	}
	return nil
}

func (r *Repo) SetReplayVideo(ctx context.Context, id, videoID string) error {
	sql, args, err := r.Builder.
		Update("live_sessions").
		Set("replay_video_id", videoID).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("LiveSessionRepo - SetReplayVideo - Builder: %w", err)
	}

	if _, err := r.Pool.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("LiveSessionRepo - SetReplayVideo - Exec: %w", err)
	}
	return nil
}
