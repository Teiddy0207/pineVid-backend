-- Opaque, rotating refresh tokens. Only a SHA-256 hash is stored, so a DB
-- leak doesn't hand out usable tokens. Each refresh revokes the presented
-- token and issues a new one (replaced_by links the chain); presenting an
-- already-revoked token is treated as theft and revokes every token of that
-- user.
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash CHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    revoked_at TIMESTAMP WITH TIME ZONE,
    replaced_by UUID,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);
