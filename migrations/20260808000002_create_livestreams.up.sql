CREATE TABLE IF NOT EXISTS livestreams (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL UNIQUE,
    stream_key VARCHAR(128) NOT NULL UNIQUE,
    title VARCHAR(255) NOT NULL DEFAULT 'Untitled Livestream',
    category VARCHAR(64) DEFAULT 'General',
    is_live BOOLEAN NOT NULL DEFAULT FALSE,
    hls_url VARCHAR(512),
    viewers_count BIGINT DEFAULT 0,
    started_at TIMESTAMP WITH TIME ZONE,
    ended_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_livestreams_user_id ON livestreams(user_id);
CREATE INDEX IF NOT EXISTS idx_livestreams_stream_key ON livestreams(stream_key);
CREATE INDEX IF NOT EXISTS idx_livestreams_is_live ON livestreams(is_live);
