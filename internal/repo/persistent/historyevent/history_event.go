package historyevent

import (
 "context"
 "fmt"
 "time"

 redispkg "github.com/evrone/go-clean-template/pkg/redis"
 "github.com/redis/go-redis/v9"
)

// Do not MAXLEN-trim unprocessed history: the worker deletes only after ACK.
const StreamKey = "watch:events"
type Publisher struct { client *redis.Client }
func New(r *redispkg.Redis) *Publisher { return &Publisher{client:r.Client} }
func (p *Publisher) RecordWatch(ctx context.Context, userID, videoID string, seconds int) error {
 if err := p.client.XAdd(ctx,&redis.XAddArgs{Stream:StreamKey,Values:map[string]interface{}{
  "v":"1", "user_id":userID, "video_id":videoID, "seconds":seconds, "ts":time.Now().UTC().UnixMilli(),
 }}).Err(); err != nil { return fmt.Errorf("publish watch history: %w",err) }
 return nil
}
