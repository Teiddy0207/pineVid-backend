CREATE TABLE IF NOT EXISTS video_subtitles (
    id VARCHAR(64) PRIMARY KEY,
    video_id VARCHAR(64) NOT NULL,
    seq INT NOT NULL,
    start_sec DOUBLE PRECISION NOT NULL,
    end_sec DOUBLE PRECISION NOT NULL,
    text_en TEXT NOT NULL DEFAULT '',
    text_vi TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_video_subtitles_video_id ON video_subtitles(video_id, seq);
