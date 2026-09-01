DROP INDEX IF EXISTS idx_comments_parent_id;
ALTER TABLE comments DROP COLUMN IF EXISTS parent_id;
