-- When the current transcode job was confirmed stored by NATS JetStream.
-- NULL while status = processing means the job may never have been
-- published (API died between MarkQueued and the publish): the backend's
-- reconcile loop republishes it with the same job_id / Nats-Msg-Id.
ALTER TABLE videos ADD COLUMN job_published_at TIMESTAMPTZ;
-- Rows queued before this column existed were published by the old code path.
UPDATE videos SET job_published_at = queued_at WHERE current_job_id IS NOT NULL AND job_published_at IS NULL;
