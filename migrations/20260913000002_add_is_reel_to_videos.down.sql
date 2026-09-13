DROP INDEX IF EXISTS idx_videos_is_reel;
ALTER TABLE videos DROP COLUMN IF EXISTS is_reel;
