-- Idempotency log for the view-counter worker (worker/module/view_sync.go).
-- A batch of Redis view counts is applied to videos.views in the same
-- transaction that inserts its batch_id here, so a batch retried after a
-- crash is detected and never counted twice.
CREATE TABLE IF NOT EXISTS view_sync_batches (
    batch_id VARCHAR(64) PRIMARY KEY,
    video_count INT NOT NULL,
    view_count BIGINT NOT NULL,
    applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
