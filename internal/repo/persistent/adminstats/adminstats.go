// Package adminstats implements cross-domain aggregate queries (spanning
// videos, livestreams and users) for the Admin Dashboard. These queries
// don't belong to any single domain repo (VideoRepo/LivestreamRepo/UserRepo),
// so they get their own small repo instead of being bolted onto one of those.
package adminstats

import (
	"context"
	"fmt"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/pkg/postgres"
)

type Repo struct {
	*postgres.Postgres
}

func New(pg *postgres.Postgres) repo.AdminStatsRepo {
	return &Repo{pg}
}

// ChannelStats counts "channels" — users who have published at least one
// video or gone live at least once — split into total/currently-active/
// banned. A raw CTE query is used here rather than squirrel, which doesn't
// have first-class support for CTEs.
func (r *Repo) ChannelStats(ctx context.Context) (entity.ChannelStats, error) {
	const query = `
		WITH channel_users AS (
			SELECT user_id FROM videos
			UNION
			SELECT user_id FROM livestreams WHERE started_at IS NOT NULL
		)
		SELECT
			(SELECT COUNT(*) FROM channel_users),
			(SELECT COUNT(DISTINCT user_id) FROM livestreams WHERE is_live = true),
			(SELECT COUNT(DISTINCT c.user_id) FROM channel_users c
				JOIN users u ON u.id::text = c.user_id WHERE u.is_banned)
	`

	var stats entity.ChannelStats
	if err := r.Pool.QueryRow(ctx, query).Scan(&stats.TotalChannels, &stats.ActiveChannels, &stats.BannedChannels); err != nil {
		return entity.ChannelStats{}, fmt.Errorf("AdminStatsRepo - ChannelStats - QueryRow: %w", err)
	}

	return stats, nil
}

// CategoryBreakdown returns the number of videos in each category, most
// populous first.
func (r *Repo) CategoryBreakdown(ctx context.Context) ([]entity.CategoryStat, error) {
	sql, args, err := r.Builder.
		Select("category", "COUNT(*) AS video_count").
		From("videos").
		GroupBy("category").
		OrderBy("video_count DESC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("AdminStatsRepo - CategoryBreakdown - r.Builder: %w", err)
	}

	rows, err := r.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("AdminStatsRepo - CategoryBreakdown - Query: %w", err)
	}
	defer rows.Close()

	stats := make([]entity.CategoryStat, 0)
	for rows.Next() {
		var s entity.CategoryStat
		if err := rows.Scan(&s.Category, &s.VideoCount); err != nil {
			return nil, fmt.Errorf("AdminStatsRepo - CategoryBreakdown - Scan: %w", err)
		}
		stats = append(stats, s)
	}

	return stats, nil
}
