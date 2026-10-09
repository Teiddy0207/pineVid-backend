-- Idempotency log for transcode webhooks / DLQ messages
-- (docs/message-contract.md §4.3). The key is <job_id>:<STATUS>; it is
-- inserted in the same transaction that applies the event, so a redelivered
-- or concurrently repeated event becomes a no-op.
CREATE TABLE IF NOT EXISTS processed_webhooks (
    idempotency_key VARCHAR(128) PRIMARY KEY,
    job_id VARCHAR(64) NOT NULL,
    video_id VARCHAR(64) NOT NULL,
    status VARCHAR(16) NOT NULL,
    -- applied | stale_job | invalid_transition
    outcome VARCHAR(32) NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_processed_webhooks_job_id ON processed_webhooks(job_id);
