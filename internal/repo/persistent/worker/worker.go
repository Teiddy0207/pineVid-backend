package worker

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/pkg/postgres"
)

type Repo struct {
	*postgres.Postgres
}

func New(pg *postgres.Postgres) repo.WorkerRepo {
	return &Repo{pg}
}

// UpsertHeartbeat records/updates a transcode worker's latest self-reported status.
func (r *Repo) UpsertHeartbeat(ctx context.Context, hb entity.WorkerHeartbeat) error {
	sql, args, err := r.Builder.
		Insert("worker_heartbeats").
		Columns("worker_id", "status", "current_job", "cpu_usage", "ram_usage_mb", "last_heartbeat").
		Values(hb.WorkerID, hb.Status, hb.CurrentJob, hb.CPUUsage, hb.RAMUsageMB, sq.Expr("CURRENT_TIMESTAMP")).
		Suffix(`ON CONFLICT (worker_id) DO UPDATE SET
			status = EXCLUDED.status,
			current_job = EXCLUDED.current_job,
			cpu_usage = EXCLUDED.cpu_usage,
			ram_usage_mb = EXCLUDED.ram_usage_mb,
			last_heartbeat = EXCLUDED.last_heartbeat`).
		ToSql()
	if err != nil {
		return fmt.Errorf("WorkerRepo - UpsertHeartbeat - r.Builder: %w", err)
	}

	if _, err := r.Pool.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("WorkerRepo - UpsertHeartbeat - Exec: %w", err)
	}
	return nil
}

// ListActive returns workers whose last heartbeat is within staleAfter of now
// (workers that stopped reporting — e.g. crashed/killed — simply age out).
func (r *Repo) ListActive(ctx context.Context, staleAfter time.Duration) ([]entity.WorkerHeartbeat, error) {
	cutoff := time.Now().UTC().Add(-staleAfter)

	sql, args, err := r.Builder.
		Select("worker_id", "status", "current_job", "cpu_usage", "ram_usage_mb", "last_heartbeat").
		From("worker_heartbeats").
		Where(sq.GtOrEq{"last_heartbeat": cutoff}).
		OrderBy("worker_id ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("WorkerRepo - ListActive - r.Builder: %w", err)
	}

	rows, err := r.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("WorkerRepo - ListActive - Query: %w", err)
	}
	defer rows.Close()

	workers := make([]entity.WorkerHeartbeat, 0)
	for rows.Next() {
		var hb entity.WorkerHeartbeat
		if err := rows.Scan(&hb.WorkerID, &hb.Status, &hb.CurrentJob, &hb.CPUUsage, &hb.RAMUsageMB, &hb.LastHeartbeat); err != nil {
			return nil, fmt.Errorf("WorkerRepo - ListActive - rows.Scan: %w", err)
		}
		workers = append(workers, hb)
	}

	return workers, nil
}
