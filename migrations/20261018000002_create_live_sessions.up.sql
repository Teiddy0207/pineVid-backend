-- Each live broadcast is tracked as an independent session so that title,
-- thumbnail, chat history, and the eventual replay video are all scoped to
-- that specific broadcast rather than shared across the streamer's channel row.
-- livestreams holds stream-key config; live_sessions holds per-broadcast data.
CREATE TABLE IF NOT EXISTS live_sessions (
    id              VARCHAR(64)  PRIMARY KEY,
    livestream_id   VARCHAR(64)  NOT NULL REFERENCES livestreams(id) ON DELETE CASCADE,
    title           VARCHAR(255) NOT NULL DEFAULT '',
    thumbnail_url   VARCHAR(512) NOT NULL DEFAULT '',
    category        VARCHAR(64)  NOT NULL DEFAULT 'General',
    started_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    ended_at        TIMESTAMPTZ,
    -- Filled in by processDVR once the transcode job is queued.
    replay_video_id VARCHAR(64)  REFERENCES videos(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Most common access pattern: latest sessions for a given channel.
CREATE INDEX IF NOT EXISTS idx_live_sessions_livestream_started
    ON live_sessions(livestream_id, started_at DESC);

-- Used by Studio replay tab to look up which session a video came from.
CREATE INDEX IF NOT EXISTS idx_live_sessions_replay_video
    ON live_sessions(replay_video_id);
