-- TOTP anti-replay. Records the last 30-second period index a code was
-- accepted for, per user. Login (and confirm/disable) only accept a code whose
-- period is strictly newer than this, so a code observed in transit can't be
-- replayed within its validity window. NULL means no code consumed yet.
ALTER TABLE user_totp ADD COLUMN IF NOT EXISTS last_used_period BIGINT;
