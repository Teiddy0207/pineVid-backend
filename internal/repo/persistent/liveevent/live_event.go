package liveevent

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	redispkg "github.com/evrone/go-clean-template/pkg/redis"
	"github.com/redis/go-redis/v9"
)

// Keys shared with the live-agg worker (live-agg/module) and documented in
// docs/message-contract.md.
const (
	StreamKey = "live:events"
	// StreamMaxLen caps the stream (approximate trim). Entries are only
	// needed until live-agg acks them; this keeps hours of backlog if the
	// worker is down without letting Redis grow without bound.
	StreamMaxLen = 200000
	// SchemaVersion is the "v" field of every entry.
	SchemaVersion    = "1"
	heartTotalKeyFmt = "stream_hearts:%s"
)

// heartScript bumps the running total and appends the event in one atomic
// step, so the realtime total and the durable log can't drift apart.
// KEYS[1] = total key, KEYS[2] = stream; ARGV[1] = maxlen, ARGV[2..] = fields.
var heartScript = redis.NewScript(`
local total = redis.call('INCR', KEYS[1])
local args = {'XADD', KEYS[2], 'MAXLEN', '~', ARGV[1], '*'}
for i = 2, #ARGV do args[#args + 1] = ARGV[i] end
redis.call(unpack(args))
return total
`)

type Repo struct {
	rdb *redis.Client
}

// New returns nil when Redis is unavailable so callers can fall back
// (a nil *Repo must not become a non-nil interface).
func New(r *redispkg.Redis) repo.LiveEventLog {
	if r == nil || r.Client == nil {
		return nil
	}
	return &Repo{rdb: r.Client}
}

func fields(ev entity.LiveEvent) []interface{} {
	return []interface{}{
		"v", SchemaVersion,
		"type", ev.Type,
		"livestream_id", ev.LivestreamID,
		"user_id", ev.UserID,
		"username", ev.Username,
		"avatar", ev.Avatar,
		"text", ev.Text,
		"ts", strconv.FormatInt(ev.At.UnixMilli(), 10),
	}
}

func (r *Repo) AppendChat(ctx context.Context, ev entity.LiveEvent) error {
	err := r.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamKey,
		MaxLen: StreamMaxLen,
		Approx: true,
		Values: fields(ev),
	}).Err()
	if err != nil {
		return fmt.Errorf("LiveEventLog - AppendChat: %w: %w", entity.ErrLiveEventsUnavailable, err)
	}
	return nil
}

func (r *Repo) AppendHeart(ctx context.Context, ev entity.LiveEvent) (int64, error) {
	argv := append([]interface{}{StreamMaxLen}, fields(ev)...)
	total, err := heartScript.Run(ctx, r.rdb,
		[]string{fmt.Sprintf(heartTotalKeyFmt, ev.LivestreamID), StreamKey}, argv...).Int64()
	if err != nil {
		return 0, fmt.Errorf("LiveEventLog - AppendHeart: %w: %w", entity.ErrLiveEventsUnavailable, err)
	}
	return total, nil
}

func (r *Repo) HeartTotal(ctx context.Context, livestreamID string) (int64, error) {
	n, err := r.rdb.Get(ctx, fmt.Sprintf(heartTotalKeyFmt, livestreamID)).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("LiveEventLog - HeartTotal: %w", err)
	}
	return n, nil
}

func (r *Repo) ResetHearts(ctx context.Context, livestreamID string) error {
	if err := r.rdb.Del(ctx, fmt.Sprintf(heartTotalKeyFmt, livestreamID)).Err(); err != nil {
		return fmt.Errorf("LiveEventLog - ResetHearts: %w", err)
	}
	return nil
}
