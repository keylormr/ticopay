-- Session revocation: every issued JWT carries the user's token_version. A
-- password reset (and any future "log out everywhere") bumps this counter,
-- which instantly invalidates all previously-issued access/refresh tokens.
ALTER TABLE users ADD COLUMN IF NOT EXISTS token_version INTEGER NOT NULL DEFAULT 0;
