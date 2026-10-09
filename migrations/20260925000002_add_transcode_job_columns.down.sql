ALTER TABLE videos DROP COLUMN IF EXISTS transcode_completed_at;
ALTER TABLE videos DROP COLUMN IF EXISTS transcode_started_at;
ALTER TABLE videos DROP COLUMN IF EXISTS queued_at;
ALTER TABLE videos DROP COLUMN IF EXISTS current_job_id;
