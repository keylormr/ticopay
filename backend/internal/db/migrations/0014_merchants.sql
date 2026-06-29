-- Commerce rail: a user can register one or more merchants. A merchant starts
-- `pending` and only a `verified` merchant can emit QR charges. Charges reuse
-- payment_requests (linked by merchant_id) and carry a commission.
CREATE TABLE IF NOT EXISTS merchants (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    category       TEXT NOT NULL DEFAULT '',
    legal_name     TEXT NOT NULL DEFAULT '',
    id_type        TEXT,                                  -- fisica | juridica | dimex
    id_number      TEXT,
    status         TEXT NOT NULL DEFAULT 'pending',       -- pending | verified | rejected
    reject_reason  TEXT NOT NULL DEFAULT '',
    commission_bps INT  NOT NULL DEFAULT 50 CHECK (commission_bps >= 0 AND commission_bps <= 10000),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_merchants_owner  ON merchants(owner_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_merchants_pending ON merchants(created_at DESC) WHERE status = 'pending';

-- A cobro can belong to a merchant; merchant cobros carry a commission.
ALTER TABLE payment_requests ADD COLUMN IF NOT EXISTS merchant_id UUID REFERENCES merchants(id) ON DELETE SET NULL;
