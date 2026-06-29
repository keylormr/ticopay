-- Back-office user lifecycle: deactivation. A disabled user keeps their data
-- but cannot authenticate (checked in requireAuth). Roles are validated in
-- application code; valid values: user | merchant | support | analyst | admin.
ALTER TABLE users ADD COLUMN IF NOT EXISTS disabled BOOLEAN NOT NULL DEFAULT false;
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role) WHERE role <> 'user';
