-- 0002_session_revocation.sql
-- Harden session lifecycle by storing hashed tokens, tracking revocation, and indexing lookups.

ALTER TABLE sessions
  ADD COLUMN token_hash TEXT,
  ADD COLUMN revoked_at TIMESTAMPTZ;

UPDATE sessions
  SET token_hash = encode(digest(jwt, 'sha256'), 'hex')
  WHERE token_hash IS NULL;

ALTER TABLE sessions
  ALTER COLUMN token_hash SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_token_hash ON sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_sessions_user_active ON sessions(user_id) WHERE revoked_at IS NULL;

ALTER TABLE sessions
  DROP COLUMN jwt;
