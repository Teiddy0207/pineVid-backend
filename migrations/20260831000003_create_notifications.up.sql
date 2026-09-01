CREATE TABLE IF NOT EXISTS notifications (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    sender_id VARCHAR(64) NOT NULL DEFAULT '',
    sender_name VARCHAR(128) NOT NULL DEFAULT '',
    sender_avatar VARCHAR(512) NOT NULL DEFAULT '',
    type VARCHAR(32) NOT NULL,
    title VARCHAR(255) NOT NULL DEFAULT '',
    message VARCHAR(1000) NOT NULL DEFAULT '',
    target_url VARCHAR(512) NOT NULL DEFAULT '',
    is_read BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_user_created ON notifications(user_id, created_at DESC);
