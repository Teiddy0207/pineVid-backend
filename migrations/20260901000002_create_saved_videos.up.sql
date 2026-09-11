CREATE TABLE IF NOT EXISTS saved_videos (
    id VARCHAR(64) PRIMARY KEY,
    video_id VARCHAR(64) NOT NULL,
    user_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT unique_user_video_save UNIQUE (user_id, video_id)
);

CREATE INDEX IF NOT EXISTS idx_saved_videos_video_id ON saved_videos(video_id);
CREATE INDEX IF NOT EXISTS idx_saved_videos_user_id ON saved_videos(user_id);
