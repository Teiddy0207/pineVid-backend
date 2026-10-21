-- Chat messages are persisted per live session so that viewers can read
-- the replay chat after the broadcast ends. The ChatHub still broadcasts
-- in-memory for real-time delivery; this table is the durable store.
CREATE TABLE IF NOT EXISTS live_chat_messages (
    id         BIGSERIAL    PRIMARY KEY,
    session_id VARCHAR(64)  NOT NULL REFERENCES live_sessions(id) ON DELETE CASCADE,
    username   VARCHAR(128) NOT NULL,
    avatar     VARCHAR(512) NOT NULL DEFAULT '',
    text       TEXT         NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Ordered read for replay: retrieve messages of a session in chronological order.
CREATE INDEX IF NOT EXISTS idx_live_chat_messages_session_created
    ON live_chat_messages(session_id, created_at);
