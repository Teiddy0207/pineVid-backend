CREATE TABLE IF NOT EXISTS watch_history (
    user_id VARCHAR(64) NOT NULL,
    video_id VARCHAR(64) NOT NULL,
    watch_seconds INT NOT NULL DEFAULT 0,
    last_watched_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, video_id)
);

CREATE INDEX IF NOT EXISTS idx_watch_history_user_last_watched ON watch_history(user_id, last_watched_at DESC);
