package video

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

func New(pg *postgres.Postgres) repo.VideoRepo {
	return &Repo{pg}
}

// videoColumns is the select list every read shares; scanVideo scans it in
// the same order.
var videoColumns = []string{
	"v.id", "v.user_id", "COALESCE(u.username, u.email, 'Creator')", "COALESCE(u.avatar_url, '')",
	"v.title", "v.description", "v.category", "v.status", "v.visibility", "v.raw_s3_key", "v.hls_url",
	"v.thumbnail_url", "v.duration", "v.is_reel", "v.views", "v.created_at", "v.updated_at",
	"COALESCE(v.current_job_id, '')", "v.queued_at", "v.transcode_started_at", "v.transcode_completed_at",
}

type scanner interface {
	Scan(dest ...any) error
}

func scanVideo(row scanner) (entity.Video, error) {
	var v entity.Video
	err := row.Scan(&v.ID, &v.UserID, &v.UserName, &v.UserAvatar, &v.Title, &v.Description, &v.Category, &v.Status,
		&v.Visibility, &v.RawS3Key, &v.HLSUrl, &v.ThumbnailUrl, &v.Duration, &v.IsReel, &v.Views, &v.CreatedAt,
		&v.UpdatedAt, &v.CurrentJobID, &v.QueuedAt, &v.TranscodeStartedAt, &v.TranscodeCompletedAt)
	return v, err
}

func (r *Repo) Store(ctx context.Context, v *entity.Video) error {
	sql, args, err := r.Builder.
		Insert("videos").
		Columns("id", "user_id", "title", "description", "category", "status", "visibility", "raw_s3_key", "hls_url", "thumbnail_url", "duration", "is_reel", "views", "created_at", "updated_at").
		Values(v.ID, v.UserID, v.Title, v.Description, v.Category, v.Status, v.Visibility, v.RawS3Key, v.HLSUrl, v.ThumbnailUrl, v.Duration, v.IsReel, v.Views, v.CreatedAt, v.UpdatedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("VideoRepo - Store - r.Builder: %w", err)
	}

	_, err = r.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("VideoRepo - Store - r.Pool.Exec: %w", err)
	}

	return nil
}

func (r *Repo) GetByID(ctx context.Context, id string) (entity.Video, error) {
	sql, args, err := r.Builder.
		Select(videoColumns...).
		From("videos v").
		LeftJoin("users u ON v.user_id::text = u.id::text").
		Where(sq.Eq{"v.id": id}).
		ToSql()
	if err != nil {
		return entity.Video{}, fmt.Errorf("VideoRepo - GetByID - r.Builder: %w", err)
	}

	v, err := scanVideo(r.Pool.QueryRow(ctx, sql, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Video{}, entity.ErrVideoNotFound
		}
		return entity.Video{}, fmt.Errorf("VideoRepo - GetByID - QueryRow: %w", err)
	}

	// Replay metadata is only needed on the watch page, not in video listings.
	err = r.Pool.QueryRow(ctx, `SELECT id, started_at FROM live_sessions
		WHERE replay_video_id = $1 ORDER BY started_at DESC LIMIT 1`, id).
		Scan(&v.ReplaySessionID, &v.ReplayStartedAt)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return entity.Video{}, fmt.Errorf("VideoRepo - GetByID - replay: %w", err)
	}
	return v, nil
}

// GetByIDs fetches multiple videos by ID in one query, restricted to
// public/complete ones (the only kind a "trending" ranking should ever
// surface). Order is not guaranteed to match ids — callers that need a
// specific ranking (e.g. by trending score) must re-sort the result
// themselves.
func (r *Repo) GetByIDs(ctx context.Context, ids []string) ([]entity.Video, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	sql, args, err := r.Builder.
		Select(videoColumns...).
		From("videos v").
		LeftJoin("users u ON v.user_id::text = u.id::text").
		Where(sq.Eq{"v.id": ids}).
		Where(sq.Eq{"v.status": entity.VideoStatusComplete}).
		Where(sq.Eq{"v.visibility": entity.VideoVisibilityPublic}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("VideoRepo - GetByIDs - r.Builder: %w", err)
	}

	rows, err := r.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("VideoRepo - GetByIDs - Query: %w", err)
	}
	defer rows.Close()

	videos := make([]entity.Video, 0, len(ids))
	for rows.Next() {
		v, err := scanVideo(rows)
		if err != nil {
			return nil, fmt.Errorf("VideoRepo - GetByIDs - rows.Scan: %w", err)
		}
		videos = append(videos, v)
	}

	return videos, nil
}

func (r *Repo) List(ctx context.Context, filter repo.VideoFilter) ([]entity.Video, int, error) {
	countBuilder := r.Builder.Select("COUNT(*)").From("videos v").LeftJoin("users u ON v.user_id::text = u.id::text")

	if filter.UserID != "" {
		countBuilder = countBuilder.Where(sq.Eq{"v.user_id": filter.UserID})
	}
	if filter.Category != "" {
		countBuilder = countBuilder.Where(sq.Eq{"v.category": filter.Category})
	}
	if filter.Query != "" {
		pat := "%" + filter.Query + "%"
		countBuilder = countBuilder.Where(sq.Or{
			sq.ILike{"v.title": pat},
			sq.ILike{"v.description": pat},
			sq.ILike{"v.category": pat},
			sq.ILike{"u.username": pat},
		})
	}
	if filter.Status != nil {
		countBuilder = countBuilder.Where(sq.Eq{"v.status": *filter.Status})
	}
	if filter.Visibility != nil {
		countBuilder = countBuilder.Where(sq.Eq{"v.visibility": *filter.Visibility})
	}
	if filter.IsReel != nil {
		countBuilder = countBuilder.Where(sq.Eq{"v.is_reel": *filter.IsReel})
	}

	countSQL, countArgs, err := countBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("VideoRepo - List - countBuilder: %w", err)
	}

	var total int
	err = r.Pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("VideoRepo - List - count query: %w", err)
	}

	dataBuilder := r.Builder.
		Select(videoColumns...).
		From("videos v").
		LeftJoin("users u ON v.user_id::text = u.id::text").
		OrderBy("v.created_at DESC").
		Limit(filter.Limit).
		Offset(filter.Offset)

	if filter.UserID != "" {
		dataBuilder = dataBuilder.Where(sq.Eq{"v.user_id": filter.UserID})
	}
	if filter.Category != "" {
		dataBuilder = dataBuilder.Where(sq.Eq{"v.category": filter.Category})
	}
	if filter.Query != "" {
		pat := "%" + filter.Query + "%"
		dataBuilder = dataBuilder.Where(sq.Or{
			sq.ILike{"v.title": pat},
			sq.ILike{"v.description": pat},
			sq.ILike{"v.category": pat},
			sq.ILike{"u.username": pat},
		})
	}
	if filter.Status != nil {
		dataBuilder = dataBuilder.Where(sq.Eq{"v.status": *filter.Status})
	}
	if filter.Visibility != nil {
		dataBuilder = dataBuilder.Where(sq.Eq{"v.visibility": *filter.Visibility})
	}
	if filter.IsReel != nil {
		dataBuilder = dataBuilder.Where(sq.Eq{"v.is_reel": *filter.IsReel})
	}

	dataSQL, dataArgs, err := dataBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("VideoRepo - List - dataBuilder: %w", err)
	}

	rows, err := r.Pool.Query(ctx, dataSQL, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("VideoRepo - List - Query: %w", err)
	}
	defer rows.Close()

	videos := make([]entity.Video, 0, filter.Limit)
	for rows.Next() {
		v, err := scanVideo(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("VideoRepo - List - rows.Scan: %w", err)
		}
		videos = append(videos, v)
	}

	return videos, total, nil
}

// Update saves the owner-editable fields. Status, HLS URL and view count are
// deliberately left out: they are written only by MarkQueued,
// ApplyTranscodeEvent, SetStatus and the view-sync worker, so an edit racing
// a transcode callback or a view sync can't write back a stale value.
func (r *Repo) Update(ctx context.Context, v *entity.Video) error {
	sql, args, err := r.Builder.
		Update("videos").
		Set("title", v.Title).
		Set("description", v.Description).
		Set("category", v.Category).
		Set("visibility", v.Visibility).
		Set("thumbnail_url", v.ThumbnailUrl).
		Set("duration", v.Duration).
		Set("updated_at", v.UpdatedAt).
		Where(sq.Eq{"id": v.ID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("VideoRepo - Update - r.Builder: %w", err)
	}

	_, err = r.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("VideoRepo - Update - Exec: %w", err)
	}

	return nil
}

// SetStatus force-sets a video's status (admin takedown).
func (r *Repo) SetStatus(ctx context.Context, id string, status entity.VideoStatus, at time.Time) error {
	sql, args, err := r.Builder.
		Update("videos").
		Set("status", status).
		Set("updated_at", at).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("VideoRepo - SetStatus - r.Builder: %w", err)
	}

	if _, err := r.Pool.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("VideoRepo - SetStatus - Exec: %w", err)
	}

	return nil
}

// MarkQueued moves a video from `from` to processing and makes jobID its
// current transcode job, resetting the job timestamps. It reports false
// (changing nothing) if the video was no longer in `from` — so two
// concurrent confirms can't enqueue two jobs.
func (r *Repo) MarkQueued(ctx context.Context, id, jobID string, from entity.VideoStatus, at time.Time) (bool, error) {
	sql, args, err := r.Builder.
		Update("videos").
		Set("status", entity.VideoStatusProcessing).
		Set("current_job_id", jobID).
		Set("queued_at", at).
		Set("transcode_started_at", nil).
		Set("transcode_completed_at", nil).
		Set("updated_at", at).
		Where(sq.Eq{"id": id, "status": from}).
		ToSql()
	if err != nil {
		return false, fmt.Errorf("VideoRepo - MarkQueued - r.Builder: %w", err)
	}

	tag, err := r.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return false, fmt.Errorf("VideoRepo - MarkQueued - Exec: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}

// RevertQueued undoes MarkQueued when the job could not be published, as long
// as the job never started.
func (r *Repo) RevertQueued(ctx context.Context, id, jobID string, to entity.VideoStatus, at time.Time) error {
	sql, args, err := r.Builder.
		Update("videos").
		Set("status", to).
		Set("updated_at", at).
		Where(sq.Eq{"id": id, "current_job_id": jobID, "status": entity.VideoStatusProcessing}).
		Where("transcode_started_at IS NULL").
		ToSql()
	if err != nil {
		return fmt.Errorf("VideoRepo - RevertQueued - r.Builder: %w", err)
	}

	if _, err := r.Pool.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("VideoRepo - RevertQueued - Exec: %w", err)
	}

	return nil
}

// ApplyTranscodeEvent applies one transcode event exactly once
// (docs/message-contract.md §4.3). In a single transaction it records the
// idempotency key — a key seen before makes the event a no-op — then locks
// the video row, checks the one-way state machine and applies the change.
func (r *Repo) ApplyTranscodeEvent(ctx context.Context, ev entity.TranscodeEvent, at time.Time) (entity.TranscodeEventResult, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return entity.TranscodeEventResult{}, fmt.Errorf("VideoRepo - ApplyTranscodeEvent - Begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	insSQL, insArgs, err := r.Builder.
		Insert("processed_webhooks").
		Columns("idempotency_key", "job_id", "video_id", "status", "outcome", "processed_at").
		Values(ev.IdempotencyKey, ev.JobID, ev.VideoID, ev.Status, "received", at).
		Suffix("ON CONFLICT (idempotency_key) DO NOTHING").
		ToSql()
	if err != nil {
		return entity.TranscodeEventResult{}, fmt.Errorf("VideoRepo - ApplyTranscodeEvent - insert ToSql: %w", err)
	}

	tag, err := tx.Exec(ctx, insSQL, insArgs...)
	if err != nil {
		return entity.TranscodeEventResult{}, fmt.Errorf("VideoRepo - ApplyTranscodeEvent - insert Exec: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return entity.TranscodeEventResult{Reason: entity.TranscodeSkipDuplicate}, nil
	}

	selSQL, selArgs, err := r.Builder.
		Select(videoColumns...).
		From("videos v").
		LeftJoin("users u ON v.user_id::text = u.id::text").
		Where(sq.Eq{"v.id": ev.VideoID}).
		Suffix("FOR UPDATE OF v").
		ToSql()
	if err != nil {
		return entity.TranscodeEventResult{}, fmt.Errorf("VideoRepo - ApplyTranscodeEvent - select ToSql: %w", err)
	}

	before, err := scanVideo(tx.QueryRow(ctx, selSQL, selArgs...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.TranscodeEventResult{}, entity.ErrVideoNotFound
		}
		return entity.TranscodeEventResult{}, fmt.Errorf("VideoRepo - ApplyTranscodeEvent - select: %w", err)
	}

	apply, reason := entity.DecideTranscodeTransition(before, ev)
	outcome := reason
	if apply {
		outcome = "applied"

		upd := r.Builder.Update("videos").Set("updated_at", at).Where(sq.Eq{"id": before.ID})
		switch ev.Status {
		case entity.TranscodeEventProcessing:
			upd = upd.Set("transcode_started_at", at)
		case entity.TranscodeEventCompleted:
			upd = upd.Set("status", entity.VideoStatusComplete).
				Set("hls_url", ev.HLSMasterURL).
				Set("transcode_completed_at", at)
		case entity.TranscodeEventFailed:
			upd = upd.Set("status", entity.VideoStatusFailed)
		}

		updSQL, updArgs, err := upd.ToSql()
		if err != nil {
			return entity.TranscodeEventResult{}, fmt.Errorf("VideoRepo - ApplyTranscodeEvent - update ToSql: %w", err)
		}
		if _, err := tx.Exec(ctx, updSQL, updArgs...); err != nil {
			return entity.TranscodeEventResult{}, fmt.Errorf("VideoRepo - ApplyTranscodeEvent - update Exec: %w", err)
		}
	}

	outSQL, outArgs, err := r.Builder.
		Update("processed_webhooks").
		Set("outcome", outcome).
		Where(sq.Eq{"idempotency_key": ev.IdempotencyKey}).
		ToSql()
	if err != nil {
		return entity.TranscodeEventResult{}, fmt.Errorf("VideoRepo - ApplyTranscodeEvent - outcome ToSql: %w", err)
	}
	if _, err := tx.Exec(ctx, outSQL, outArgs...); err != nil {
		return entity.TranscodeEventResult{}, fmt.Errorf("VideoRepo - ApplyTranscodeEvent - outcome Exec: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return entity.TranscodeEventResult{}, fmt.Errorf("VideoRepo - ApplyTranscodeEvent - Commit: %w", err)
	}

	return entity.TranscodeEventResult{Applied: apply, Reason: reason, Before: before}, nil
}

func (r *Repo) Delete(ctx context.Context, id string) error {
	sql, args, err := r.Builder.
		Delete("videos").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("VideoRepo - Delete - r.Builder: %w", err)
	}

	_, err = r.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("VideoRepo - Delete - Exec: %w", err)
	}

	return nil
}
