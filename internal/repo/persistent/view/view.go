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
)

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

	return true, newViews, nil
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
