-- One-time recovery codes: a passwordless account's escape hatch when every
-- passkey is lost. Codes are bcrypt-hashed (never stored in plaintext) and
-- consumed on use (used_at set), exactly like a single-use password.
CREATE TABLE IF NOT EXISTS passkey_recovery_codes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash  TEXT NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Only unused codes matter at login time; index them for the per-user lookup.
CREATE INDEX IF NOT EXISTS idx_recovery_user_unused
    ON passkey_recovery_codes(user_id) WHERE used_at IS NULL;
