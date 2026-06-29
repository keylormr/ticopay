-- Back-office audit trail. Every privileged mutation (merchant verify/reject,
-- commission change, staff create, role change, enable/disable) writes one
-- immutable row here. actor_id is the staff member who acted; target is the
-- affected entity id (user or merchant); detail carries action-specific context
-- as JSON (previous value, reason, etc.). Rows are append-only — no UPDATE/DELETE
-- path exists in the application.
CREATE TABLE IF NOT EXISTS admin_audit_log (
    id         BIGSERIAL PRIMARY KEY,
    actor_id   UUID NOT NULL REFERENCES users(id),
    action     TEXT NOT NULL,
    target     TEXT NOT NULL DEFAULT '',
    detail     JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_audit_created ON admin_audit_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_actor   ON admin_audit_log(actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_action  ON admin_audit_log(action, created_at DESC);
