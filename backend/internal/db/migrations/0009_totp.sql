-- TOTP 2FA (RFC 6238) as an alternative to passkeys. One secret per user;
-- it only gates login once confirmed (the user proved their app generates
-- valid codes). Replaced wholesale on re-setup.
CREATE TABLE IF NOT EXISTS user_totp (
    user_id    UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    secret     TEXT NOT NULL,
    confirmed  BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
