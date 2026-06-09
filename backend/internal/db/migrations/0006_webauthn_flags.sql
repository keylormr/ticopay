-- WebAuthn backup flags (BE/BS). go-webauthn validates that these are
-- consistent between registration and login, so they must be persisted.
ALTER TABLE webauthn_credentials ADD COLUMN IF NOT EXISTS backup_eligible BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE webauthn_credentials ADD COLUMN IF NOT EXISTS backup_state    BOOLEAN NOT NULL DEFAULT false;
