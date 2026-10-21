-- Cover image the streamer picks in Studio; shown in the live list and reused
-- as the thumbnail of the replay video created from the DVR recording.
ALTER TABLE livestreams ADD COLUMN IF NOT EXISTS thumbnail_url VARCHAR(512) NOT NULL DEFAULT '';
