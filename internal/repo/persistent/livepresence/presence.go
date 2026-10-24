// Package livepresence stores ephemeral viewers, never accumulated video views.
package livepresence

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/evrone/go-clean-template/internal/repo"
	redispkg "github.com/evrone/go-clean-template/pkg/redis"
	"github.com/redis/go-redis/v9"
)

// One atomic operation: expire tabs, update this tab, count distinct identities.
// Redis time is shared by all API replicas. Both keys share a Redis hash slot.
var update = redis.NewScript(`
local clock = redis.call('TIME')
local now = tonumber(clock[1])*1000 + math.floor(tonumber(clock[2])/1000)
local expired = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', now-45000)
for _,tab in ipairs(expired) do redis.call('HDEL',KEYS[2],tab) end
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now-45000)
if ARGV[1]=='heartbeat' then
 redis.call('ZADD',KEYS[1],now,ARGV[2])
 redis.call('HSET',KEYS[2],ARGV[2],ARGV[3])
elseif ARGV[1]=='leave' and redis.call('HGET',KEYS[2],ARGV[2])==ARGV[3] then
 redis.call('ZREM',KEYS[1],ARGV[2])
 redis.call('HDEL',KEYS[2],ARGV[2])
end
local seen = {}
local total = 0
for _,who in ipairs(redis.call('HVALS',KEYS[2])) do
 if not seen[who] then seen[who]=true; total=total+1 end
end
redis.call('EXPIRE',KEYS[1],90)
redis.call('EXPIRE',KEYS[2],90)
return total
`)

type Store struct{ client *redis.Client }

func New(r *redispkg.Redis) repo.LivePresence {
	if r == nil || r.Client == nil {
		return nil
	}
	return &Store{client: r.Client}
}
func (s *Store) Apply(ctx context.Context, broadcast, action, tab, identity string) (int64, error) {
	key := fmt.Sprintf("live:presence:{%x}", sha256.Sum256([]byte(broadcast)))
	return update.Run(ctx, s.client, []string{key + ":tabs", key + ":identities"}, action, tab, identity).Int64()
}
