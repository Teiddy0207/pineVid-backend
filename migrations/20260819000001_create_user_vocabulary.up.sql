CREATE TABLE IF NOT EXISTS user_vocabulary (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    word VARCHAR(255) NOT NULL,
    ipa VARCHAR(255) NOT NULL DEFAULT '',
    part_of_speech VARCHAR(64) NOT NULL DEFAULT '',
    meaning TEXT NOT NULL DEFAULT '',
    example TEXT NOT NULL DEFAULT '',
    video_id VARCHAR(64) NOT NULL DEFAULT '',
    video_title VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_user_vocabulary_user_id ON user_vocabulary(user_id, created_at DESC);
