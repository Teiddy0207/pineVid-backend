-- avatar_url predates this migration in some environments (added out-of-band),
-- so this is a safety net for fresh deployments that only ever run tracked migrations.
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url VARCHAR(512);
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_banned BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS follows (
    follower_id VARCHAR(64) NOT NULL,
    channel_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (follower_id, channel_id)
);

CREATE INDEX IF NOT EXISTS idx_follows_channel_id ON follows(channel_id);
