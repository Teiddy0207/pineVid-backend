-- Live chat and hearts now travel through the Redis Stream "live:events"
-- (see docs/message-contract.md §Redis Stream). The live-agg worker consumes
-- it with a consumer group and writes these tables in one transaction per
-- batch, so a crash between commit and XACK can't double-count anything.

-- Who sent the line (unique chatters per session) and which stream entry it
-- came from (a redelivered entry hits the unique index instead of a 2nd row).
ALTER TABLE live_chat_messages ADD COLUMN IF NOT EXISTS user_id  VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE live_chat_messages ADD COLUMN IF NOT EXISTS event_id VARCHAR(32);
CREATE UNIQUE INDEX IF NOT EXISTS uq_live_chat_messages_event_id
    ON live_chat_messages(event_id) WHERE event_id IS NOT NULL;

-- Per-broadcast totals, maintained by live-agg.
CREATE TABLE IF NOT EXISTS live_session_stats (
    session_id      VARCHAR(64) PRIMARY KEY REFERENCES live_sessions(id) ON DELETE CASCADE,
    hearts          BIGINT      NOT NULL DEFAULT 0,
    chat_messages   BIGINT      NOT NULL DEFAULT 0,
    unique_chatters INTEGER     NOT NULL DEFAULT 0,
    last_event_at   TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Idempotency log: stream entry IDs already applied. Only has to outlive the
-- redelivery window (pending entries are reclaimed after ~30 s), so live-agg
-- prunes rows older than an hour.
CREATE TABLE IF NOT EXISTS live_processed_events (
    event_id     VARCHAR(32) PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_live_processed_events_processed_at
    ON live_processed_events(processed_at);
