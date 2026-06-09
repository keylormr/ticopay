-- Passkeys (WebAuthn / FIDO2) credentials.
CREATE TABLE IF NOT EXISTS webauthn_credentials (
    id               BYTEA PRIMARY KEY,          -- raw credential id
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    public_key       BYTEA NOT NULL,
    attestation_type TEXT NOT NULL DEFAULT '',
    aaguid           BYTEA,
    sign_count       BIGINT NOT NULL DEFAULT 0,
    transports       TEXT NOT NULL DEFAULT '',
    name             TEXT NOT NULL DEFAULT 'Llave de acceso',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_webauthn_user ON webauthn_credentials(user_id);
