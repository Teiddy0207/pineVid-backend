-- Admin "ban stream" previously only flipped is_live=false, so the streamer
-- could go live again immediately. is_banned makes on_publish reject the key.
ALTER TABLE livestreams ADD COLUMN IF NOT EXISTS is_banned BOOLEAN NOT NULL DEFAULT FALSE;
