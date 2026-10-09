package view

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/evrone/go-clean-template/internal/entity"
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

	// ipCapDisabled turns off the anonymous per-IP hourly cap; only for load
	// tests, where every request comes from one IP (BENCHMARK_MODE).
	ipCapDisabled bool

	// directDB writes every accepted view straight to videos.views instead of
	// Redis INCR + batch sync. Only the control variant of the view-counter
	// experiment (VIEW_COUNTER_MODE=direct_db).
	directDB bool
}

// DisableIPCap is for BENCHMARK_MODE only.
func (r *Repo) DisableIPCap() { r.ipCapDisabled = true }

// UseDirectDB is for the VIEW_COUNTER_MODE=direct_db control experiment only.
func (r *Repo) UseDirectDB() { r.directDB = true }

func New(pg *postgres.Postgres, rdb *redispkg.Redis) *Repo {
	return &Repo{
		Postgres: pg,
		Redis:    rdb,
	}
}

const (
	// viewTicketTTL bounds how long a started playback can take to reach the
	// minimum watch time (generous: the viewer may pause).
	viewTicketTTL = 4 * time.Hour
	// maxViewsPerIPPerHour caps how many views one IP can add to one video
	// per hour, whatever device IDs it claims (X-Device-ID is client-chosen).
	// High enough for a shared NAT (dorm, office), low enough to stop a
	// script from inflating a video.
	maxViewsPerIPPerHour = 10
)

func viewTicketKey(ticket string) string { return "view_ticket:" + ticket }

// IssueViewTicket starts a view session: the returned one-time ticket can be
// redeemed for a view once minWatch has elapsed (see RedeemViewTicket).
func (r *Repo) IssueViewTicket(ctx context.Context, videoID string) (string, error) {
	if r.Redis == nil || r.Redis.Client == nil {
		return "", fmt.Errorf("ViewRepo - IssueViewTicket: redis unavailable")
	}

	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("ViewRepo - IssueViewTicket - rand.Read: %w", err)
	}
	ticket := hex.EncodeToString(raw)

	value := fmt.Sprintf("%s|%d", videoID, time.Now().UnixMilli())
	if err := r.Redis.Client.Set(ctx, viewTicketKey(ticket), value, viewTicketTTL).Err(); err != nil {
		return "", fmt.Errorf("ViewRepo - IssueViewTicket - Set: %w", err)
	}
	return ticket, nil
}

// RedeemViewTicket consumes a ticket for videoID. It fails with
// ErrViewTicketInvalid for an unknown, expired, already-used or other-video
// ticket, and with ErrViewTicketTooEarly (ticket kept, retry later) when
// less than minWatch has passed since it was issued — the 10-second rule is
// enforced on the server clock, not trusted from the client.
func (r *Repo) RedeemViewTicket(ctx context.Context, ticket, videoID string, minWatch time.Duration) error {
	if r.Redis == nil || r.Redis.Client == nil || ticket == "" {
		return entity.ErrViewTicketInvalid
	}

	key := viewTicketKey(ticket)
	value, err := r.Redis.Client.Get(ctx, key).Result()
	if err != nil {
		return entity.ErrViewTicketInvalid
	}

	parts := strings.SplitN(value, "|", 2)
	if len(parts) != 2 || parts[0] != videoID {
		return entity.ErrViewTicketInvalid
	}
	issuedMs, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return entity.ErrViewTicketInvalid
	}
	if time.Since(time.UnixMilli(issuedMs)) < minWatch {
		return entity.ErrViewTicketTooEarly
	}

	// DEL is the atomic "use once" step: of two concurrent redeems, only the
	// one that actually deletes the key wins.
	deleted, err := r.Redis.Client.Del(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("ViewRepo - RedeemViewTicket - Del: %w", err)
	}
	if deleted == 0 {
		return entity.ErrViewTicketInvalid
	}
	return nil
}

// ViewerID identifies "one person" for view dedup (proposal §4.3): the
// account when logged in, otherwise a SHA-256 of IP + User-Agent. Known
// limitation: anonymous viewers sharing a NAT IP and the same browser build
// count as one person.
func ViewerID(userID, clientIP, userAgent string) string {
	if userID != "" {
		return "u:" + userID
	}
	sum := sha256.Sum256([]byte(clientIP + "|" + userAgent))
	return "a:" + hex.EncodeToString(sum[:16])
}

// DedupWindow is the "1 view per person per video" window.
const DedupWindow = 60 * time.Second

// RecordView records a view in Redis unless the same viewer (ViewerID) was
// already counted for this video in the last DedupWindow. Anonymous viewers
// are also capped per IP and hour; logged-in viewers are already one per
// account. It reports whether the view counted and the pending (not yet
// synced) count.
func (r *Repo) RecordView(ctx context.Context, videoID, userID, clientIP, userAgent string) (bool, int64, error) {
	if r.Redis == nil || r.Redis.Client == nil {
		return r.recordViewFallback(ctx, videoID)
	}

	dedupKey := fmt.Sprintf("view_dedup:%s:%s", videoID, ViewerID(userID, clientIP, userAgent))

	// SET NX EX is atomic: of concurrent requests from one viewer, only one wins.
	isNewView, err := r.Redis.Client.SetNX(ctx, dedupKey, "1", DedupWindow).Result()
	if err != nil {
		return false, 0, fmt.Errorf("ViewRepo - RecordView - SetNX: %w", err)
	}

	if !isNewView {
		return false, r.GetPendingViewsForVideo(ctx, videoID), nil
	}

	if userID == "" && !r.ipCapDisabled {
		ipKey := fmt.Sprintf("view_ip:%s:%s", videoID, clientIP)
		ipCount, err := r.Redis.Client.Incr(ctx, ipKey).Result()
		if err != nil {
			return false, 0, fmt.Errorf("ViewRepo - RecordView - Incr ip: %w", err)
		}
		if ipCount == 1 {
			r.Redis.Client.Expire(ctx, ipKey, time.Hour)
		}
		if ipCount > maxViewsPerIPPerHour {
			return false, r.GetPendingViewsForVideo(ctx, videoID), nil
		}
	}

	if r.directDB {
		return r.recordViewDirect(ctx, videoID)
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

// recordViewDirect is the control variant: one UPDATE per accepted view, so
// concurrent views of one video queue on that row's lock.
func (r *Repo) recordViewDirect(ctx context.Context, videoID string) (bool, int64, error) {
	sql, args, err := r.Builder.
		Update("videos").
		Set("views", squirrel.Expr("views + 1")).
		Where(squirrel.Eq{"id": videoID}).
		Suffix("RETURNING views").
		ToSql()
	if err != nil {
		return false, 0, fmt.Errorf("ViewRepo - recordViewDirect - ToSql: %w", err)
	}
	var views int64
	if err := r.Pool.QueryRow(ctx, sql, args...).Scan(&views); err != nil {
		return false, 0, fmt.Errorf("ViewRepo - recordViewDirect - QueryRow: %w", err)
	}
	return true, views, nil
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

// viewSyncInflightKey is the hash the view-counter worker moves counts into
// while it writes them to Postgres (worker/module/view_sync.go).
const viewSyncInflightKey = "view_sync:inflight"

// GetPendingViewsForVideo returns views recorded in Redis but not yet synced
// to videos.views — including any the worker has claimed but not committed,
// so the displayed total never dips during a sync.
func (r *Repo) GetPendingViewsForVideo(ctx context.Context, videoID string) int64 {
	if r.Redis == nil || r.Redis.Client == nil {
		return 0
	}
	pipe := r.Redis.Client.Pipeline()
	counter := pipe.Get(ctx, fmt.Sprintf("video_views:%s", videoID))
	inflight := pipe.HGet(ctx, viewSyncInflightKey, videoID)
	_, _ = pipe.Exec(ctx)

	a, _ := strconv.ParseInt(counter.Val(), 10, 64)
	b, _ := strconv.ParseInt(inflight.Val(), 10, 64)
	return a + b
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
