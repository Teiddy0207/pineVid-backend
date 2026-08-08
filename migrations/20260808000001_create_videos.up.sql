CREATE TABLE IF NOT EXISTS videos (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(64) DEFAULT 'General',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    visibility VARCHAR(32) NOT NULL DEFAULT 'public',
    raw_s3_key VARCHAR(512),
    hls_url VARCHAR(512),
    thumbnail_url VARCHAR(512),
    duration VARCHAR(32) DEFAULT '00:00',
    views BIGINT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_videos_user_id ON videos(user_id);
CREATE INDEX IF NOT EXISTS idx_videos_status ON videos(status);
CREATE INDEX IF NOT EXISTS idx_videos_category ON videos(category);
