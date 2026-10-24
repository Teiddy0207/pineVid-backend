// Package watchprogress persists playback checkpoints through a Redis Stream.
package watchprogress

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/evrone/go-clean-template/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
)

const Stream = "watch:progress:events"
const group = "watch-progress-db"

type Progress struct {
	Duration  float64 `json:"duration_seconds"`
	Position  float64 `json:"position_seconds"`
	Completed bool    `json:"completed"`
	Version   string  `json:"version"`
}
type Database interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
type Service struct {
	Redis *redis.Client
	PG    Database
}

func New(r *redis.Client, pg *postgres.Postgres) *Service { return &Service{r, pg.Pool} }
func key(user, video string) string                       { return "watch:progress:" + user + ":" + video }

// The Stream ID is a durable, server-ordered version. Never trim pending events.
var publish = redis.NewScript(`
local id = redis.call('XADD', KEYS[1], '*', 'user_id', ARGV[1], 'video_id', ARGV[2], 'position', ARGV[3], 'completed', ARGV[4], 'duration', ARGV[5])
local value = cjson.encode({position_seconds=tonumber(ARGV[3]), completed=ARGV[4]=='true', duration_seconds=tonumber(ARGV[5]), version=id})
redis.call('SET', KEYS[2], value, 'EX', 604800)
return id
`)

func (s *Service) Save(ctx context.Context, user, video string, p Progress) error {
	return publish.Run(ctx, s.Redis, []string{Stream, key(user, video)}, user, video, p.Position, fmt.Sprint(p.Completed), p.Duration).Err()
}
func (s *Service) Load(ctx context.Context, user, video string) (Progress, error) {
	var p Progress
	raw, err := s.Redis.Get(ctx, key(user, video)).Result()
	if err == nil {
		err = json.Unmarshal([]byte(raw), &p)
		if err == nil {
			return p, nil
		}
	}
	// A Redis outage must not prevent playback of a persisted checkpoint.
	err = s.PG.QueryRow(ctx, `SELECT position_seconds, completed, progress_version, playback_duration_seconds FROM watch_history WHERE user_id=$1 AND video_id=$2`, user, video).Scan(&p.Position, &p.Completed, &p.Version, &p.Duration)
	if errors.Is(err, pgx.ErrNoRows) {
		return Progress{}, nil
	}
	return p, err
}

// Stream IDs compare numerically, including the sequence within a millisecond.
const upsert = `INSERT INTO watch_history(user_id,video_id,position_seconds,completed,progress_version,playback_duration_seconds)
VALUES($1,$2,$3,$4,$5,$6)
ON CONFLICT(user_id,video_id) DO UPDATE SET position_seconds=EXCLUDED.position_seconds,
 completed=EXCLUDED.completed, playback_duration_seconds=GREATEST(watch_history.playback_duration_seconds,EXCLUDED.playback_duration_seconds), progress_version=EXCLUDED.progress_version, last_watched_at=CURRENT_TIMESTAMP
WHERE (split_part(watch_history.progress_version,'-',1)::bigint, split_part(watch_history.progress_version,'-',2)::bigint)
 < (split_part(EXCLUDED.progress_version,'-',1)::bigint, split_part(EXCLUDED.progress_version,'-',2)::bigint)`

func (s *Service) handle(ctx context.Context, m redis.XMessage) error {
	field := func(k string) string { return fmt.Sprint(m.Values[k]) }
	position, err := strconv.ParseFloat(field("position"), 64)
	if err != nil || math.IsNaN(position) || math.IsInf(position, 0) || position < 0 {
		return fmt.Errorf("invalid position in event %s", m.ID)
	}
	completed, err := strconv.ParseBool(field("completed"))
	if err != nil {
		return err
	}
	duration := 0.0
	if raw, ok := m.Values["duration"]; ok {
		var parseErr error
		duration, parseErr = strconv.ParseFloat(fmt.Sprint(raw), 64)
		if parseErr != nil || math.IsNaN(duration) || math.IsInf(duration, 0) || duration < 0 {
			return fmt.Errorf("invalid duration in event %s", m.ID)
		}
	}
	if _, err := s.PG.Exec(ctx, upsert, field("user_id"), field("video_id"), position, completed, m.ID, duration); err != nil {
		return err
	}
	// Only one persistence group owns this stream. ACK and delete atomically.
	_, err = s.Redis.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.XAck(ctx, Stream, group, m.ID)
		p.XDel(ctx, Stream, m.ID)
		return nil
	})
	return err
}
func (s *Service) Run(ctx context.Context, report func(error)) {
	consumer := uuid.NewString()
	cursor := "0-0"
	for ctx.Err() == nil {
		err := s.Redis.XGroupCreateMkStream(ctx, Stream, group, "0").Err()
		if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
			report(err)
			if !retry(ctx) {
				return
			}
			continue
		}
		// Recover abandoned work on every iteration, including while new events arrive.
		pending, next, err := s.Redis.XAutoClaim(ctx, &redis.XAutoClaimArgs{Stream: Stream, Group: group, Consumer: consumer, MinIdle: 30 * time.Second, Start: cursor, Count: 100}).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			report(err)
		}
		if next != "" {
			cursor = next
		}
		for _, m := range pending {
			if err := s.handle(ctx, m); err != nil {
				report(err)
			}
		}
		batches, err := s.Redis.XReadGroup(ctx, &redis.XReadGroupArgs{Group: group, Consumer: consumer, Streams: []string{Stream, ">"}, Count: 100, Block: time.Second}).Result()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			if ctx.Err() == nil {
				report(err)
			}
			if !retry(ctx) {
				return
			}
			continue
		}
		for _, b := range batches {
			for _, m := range b.Messages {
				if err := s.handle(ctx, m); err != nil {
					report(err)
				}
			}
		}
	}
}
func retry(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(time.Second):
		return true
	}
}
