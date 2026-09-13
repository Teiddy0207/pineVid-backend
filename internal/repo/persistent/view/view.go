package view

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/evrone/go-clean-template/pkg/postgres"
	redispkg "github.com/evrone/go-clean-template/pkg/redis"
	"github.com/redis/go-redis/v9"
)

// trendingKeyTTL keeps each daily bucket around for a bit longer than a
// week, so a "last 7 days" query never reads a bucket that already expired
// out from under it.
const trendingKeyTTL = 9 * 24 * time.Hour

// trendingKey is the Redis sorted-set key for one calendar day (UTC) of
// view counts, keyed by video ID with the view count as score. Kept
// separate from the lifetime `videos.views` Postgres column so "trending"
// can rank by recent activity instead of all-time totals.
func trendingKey(day time.Time) string {
	return "trending:views:" + day.UTC().Format("2006-01-02")
}

type Repo struct {
	*postgres.Postgres
	Redis *redispkg.Redis
}

func New(pg *postgres.Postgres, rdb *redispkg.Redis) *Repo {
	return &Repo{
		Postgres: pg,
		Redis:    rdb,
	}
}

// RecordView records a new video view in Redis with IP + Device fingerprint deduplication (TTL 60s)
func (r *Repo) RecordView(ctx context.Context, videoID, clientIP, deviceID string) (bool, int64, error) {
	if r.Redis == nil || r.Redis.Client == nil {
		return r.recordViewFallback(ctx, videoID)
	}

	// Create IP + Device Fingerprint hash
	rawFingerprint := fmt.Sprintf("%s:%s", clientIP, deviceID)
	hash := sha256.Sum256([]byte(rawFingerprint))
	fpHash := hex.EncodeToString(hash[:8])

	dedupKey := fmt.Sprintf("view_dedup:%s:%s", videoID, fpHash)

	// Try setting dedup key with 60-second TTL
	isNewView, err := r.Redis.Client.SetNX(ctx, dedupKey, "1", 60*time.Second).Result()
	if err != nil {
		return false, 0, fmt.Errorf("ViewRepo - RecordView - SetNX: %w", err)
	}

	if !isNewView {
		// Duplicate view within 60s window; get current pending views
		countStr, _ := r.Redis.Client.Get(ctx, fmt.Sprintf("video_views:%s", videoID)).Result()
		currentViews, _ := strconv.ParseInt(countStr, 10, 64)
		return false, currentViews, nil
	}

	// Increment pending view counter in Redis
	viewKey := fmt.Sprintf("video_views:%s", videoID)
	newViews, err := r.Redis.Client.Incr(ctx, viewKey).Result()
	if err != nil {
		return false, 0, fmt.Errorf("ViewRepo - RecordView - Incr: %w", err)
	}

	// Best-effort: also tally today's bucket for the trending feed. Never
	// fails the actual view count over this — trending is a nice-to-have.
	today := trendingKey(time.Now())
	pipe := r.Redis.Client.Pipeline()
	pipe.ZIncrBy(ctx, today, 1, videoID)
	pipe.Expire(ctx, today, trendingKeyTTL)
	_, _ = pipe.Exec(ctx)

	return true, newViews, nil
}

// GetTrendingVideoIDs returns up to `limit` video IDs ranked by view count
// over the last `days` calendar days (UTC), most-viewed first. Backs the
// "trending" feed — a ranking over recent activity, distinct from the
// lifetime total in Postgres's `videos.views`.
func (r *Repo) GetTrendingVideoIDs(ctx context.Context, days, limit int) ([]string, error) {
	if r.Redis == nil || r.Redis.Client == nil || days <= 0 || limit <= 0 {
		return nil, nil
	}

	now := time.Now()
	keys := make([]string, days)
	for i := 0; i < days; i++ {
		keys[i] = trendingKey(now.AddDate(0, 0, -i))
	}

	rankKey := keys[0]
	if len(keys) > 1 {
		rankKey = fmt.Sprintf("trending:union:%d", now.UnixNano())
		if err := r.Redis.Client.ZUnionStore(ctx, rankKey, &redis.ZStore{Keys: keys}).Err(); err != nil {
			return nil, fmt.Errorf("ViewRepo - GetTrendingVideoIDs - ZUnionStore: %w", err)
		}
		defer r.Redis.Client.Del(ctx, rankKey)
	}

	ids, err := r.Redis.Client.ZRevRange(ctx, rankKey, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("ViewRepo - GetTrendingVideoIDs - ZRevRange: %w", err)
	}
	return ids, nil
}

// GetPendingViewsForVideo returns real-time pending views for a video in Redis
func (r *Repo) GetPendingViewsForVideo(ctx context.Context, videoID string) int64 {
	if r.Redis == nil || r.Redis.Client == nil {
		return 0
	}
	countStr, err := r.Redis.Client.Get(ctx, fmt.Sprintf("video_views:%s", videoID)).Result()
	if err != nil {
		return 0
	}
	cnt, _ := strconv.ParseInt(countStr, 10, 64)
	return cnt
}

// GetPendingViews scans all pending view counts in Redis
func (r *Repo) GetPendingViews(ctx context.Context) (map[string]int64, error) {
	if r.Redis == nil || r.Redis.Client == nil {
		return map[string]int64{}, nil
	}

	pending := make(map[string]int64)
	var cursor uint64
	var keys []string
	var err error

	for {
		keys, cursor, err = r.Redis.Client.Scan(ctx, cursor, "video_views:*", 100).Result()
		if err != nil {
			return nil, fmt.Errorf("ViewRepo - GetPendingViews - Scan: %w", err)
		}

		for _, key := range keys {
			videoID := strings.TrimPrefix(key, "video_views:")
			countStr, getErr := r.Redis.Client.Get(ctx, key).Result()
			if getErr == nil {
				if cnt, parseErr := strconv.ParseInt(countStr, 10, 64); parseErr == nil && cnt > 0 {
					pending[videoID] = cnt
				}
			}
		}

		if cursor == 0 {
			break
		}
	}

	return pending, nil
}

// SyncBatchViewsToPostgres syncs Redis views to Postgres in a single transaction
func (r *Repo) SyncBatchViewsToPostgres(ctx context.Context, pendingViews map[string]int64) error {
	if len(pendingViews) == 0 {
		return nil
	}

	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ViewRepo - SyncBatchViewsToPostgres - Begin: %w", err)
	}
	defer tx.Rollback(ctx)

	for videoID, viewIncrement := range pendingViews {
		sql, args, err := r.Builder.
			Update("videos").
			Set("views", squirrel.Expr("views + ?", viewIncrement)).
			Where(squirrel.Eq{"id": videoID}).
			ToSql()
		if err != nil {
			return fmt.Errorf("ViewRepo - SyncBatchViewsToPostgres - ToSql: %w", err)
		}

		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			return fmt.Errorf("ViewRepo - SyncBatchViewsToPostgres - Exec: %w", err)
		}

		// Decrement processed amount from Redis
		if r.Redis != nil && r.Redis.Client != nil {
			viewKey := fmt.Sprintf("video_views:%s", videoID)
			r.Redis.Client.DecrBy(ctx, viewKey, viewIncrement)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("ViewRepo - SyncBatchViewsToPostgres - Commit: %w", err)
	}

	return nil
}

// recordViewFallback increments views directly in Postgres if Redis is disabled
func (r *Repo) recordViewFallback(ctx context.Context, videoID string) (bool, int64, error) {
	sql, args, err := r.Builder.
		Update("videos").
		Set("views", squirrel.Expr("views + 1")).
		Where(squirrel.Eq{"id": videoID}).
		ToSql()
	if err != nil {
		return false, 0, err
	}

	_, err = r.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return false, 0, err
	}

	return true, 1, nil
}
