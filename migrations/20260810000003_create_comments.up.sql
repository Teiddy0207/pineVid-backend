CREATE TABLE IF NOT EXISTS comments (
    id VARCHAR(64) PRIMARY KEY,
    video_id VARCHAR(64) NOT NULL,
    user_id VARCHAR(64) NOT NULL,
    user_name VARCHAR(128) NOT NULL DEFAULT 'Viewer',
    user_avatar VARCHAR(512) NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_comments_video_id ON comments(video_id);
CREATE INDEX IF NOT EXISTS idx_comments_created_at ON comments(created_at DESC);
