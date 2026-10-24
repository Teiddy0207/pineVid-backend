-- Durable, distinct viewers per broadcast. Concurrent presence remains in Redis.
CREATE TABLE IF NOT EXISTS live_session_viewers (
    session_id VARCHAR(64) NOT NULL REFERENCES live_sessions(id) ON DELETE CASCADE,
    identity TEXT NOT NULL,
    PRIMARY KEY (session_id, identity)
);
