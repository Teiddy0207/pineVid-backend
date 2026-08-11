CREATE TABLE IF NOT EXISTS likes (
    id VARCHAR(64) PRIMARY KEY,
    video_id VARCHAR(64) NOT NULL,
    user_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT unique_user_video_like UNIQUE (user_id, video_id)
);

CREATE INDEX IF NOT EXISTS idx_likes_video_id ON likes(video_id);
CREATE INDEX IF NOT EXISTS idx_likes_user_id ON likes(user_id);
