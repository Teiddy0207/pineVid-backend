-- Transcode job tracking: the job currently owning the video and the
-- timestamps used to measure queue wait and upload-to-playable latency.
ALTER TABLE videos ADD COLUMN IF NOT EXISTS current_job_id VARCHAR(64);
ALTER TABLE videos ADD COLUMN IF NOT EXISTS queued_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE videos ADD COLUMN IF NOT EXISTS transcode_started_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE videos ADD COLUMN IF NOT EXISTS transcode_completed_at TIMESTAMP WITH TIME ZONE;
