ALTER TABLE videos ADD COLUMN is_reel BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX idx_videos_is_reel ON videos(is_reel);
