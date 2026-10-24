ALTER TABLE watch_history ADD COLUMN playback_duration_seconds DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (playback_duration_seconds >= 0);
