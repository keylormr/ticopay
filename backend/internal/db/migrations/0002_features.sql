-- Tico Pay feature expansion: bi-currency, KYC, payment requests, vaquitas (pools).

-- 1. KYC fields on users -------------------------------------------------------
ALTER TABLE users ADD COLUMN IF NOT EXISTS id_type    TEXT;          -- 'fisica' | 'juridica' | 'dimex'
ALTER TABLE users ADD COLUMN IF NOT EXISTS id_number  TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS kyc_status TEXT NOT NULL DEFAULT 'none'; -- none | verified

-- Unique normalized phone (digits only) so transfers-by-phone are unambiguous.
CREATE UNIQUE INDEX IF NOT EXISTS uq_users_phone_norm
    ON users (regexp_replace(phone, '\D', '', 'g'))
    WHERE phone IS NOT NULL AND phone <> '';

-- 2. Transaction kind ----------------------------------------------------------
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'transfer';
-- 'transfer' | 'conversion' | 'request' | 'pool'

-- 3. Bi-currency: every user gets a USD account (back-fill existing users) ------
INSERT INTO accounts (user_id, currency, balance_cents)
SELECT id, 'USD',
       CASE email
           WHEN 'maria@ticopay.cr'  THEN 50000  -- $500.00 demo
           WHEN 'carlos@ticopay.cr' THEN 20000  -- $200.00 demo
           ELSE 0
       END
FROM users
ON CONFLICT (user_id, currency) DO NOTHING;

-- 4. Payment requests ("cobrale a alguien") -----------------------------------
CREATE TABLE IF NOT EXISTS payment_requests (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_user_id UUID REFERENCES users(id) ON DELETE SET NULL, -- optional specific payer
    amount_cents   BIGINT CHECK (amount_cents IS NULL OR amount_cents > 0),
    currency       TEXT NOT NULL DEFAULT 'CRC',
    description    TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'pending', -- pending | paid | cancelled
    paid_by        UUID REFERENCES users(id) ON DELETE SET NULL,
    paid_tx_id     UUID REFERENCES transactions(id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_req_requester ON payment_requests(requester_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_req_target    ON payment_requests(target_user_id, created_at DESC);

-- 5. Vaquitas (group savings / collection pools) ------------------------------
CREATE TABLE IF NOT EXISTS pools (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    goal_cents  BIGINT NOT NULL DEFAULT 0 CHECK (goal_cents >= 0),
    currency    TEXT NOT NULL DEFAULT 'CRC',
    status      TEXT NOT NULL DEFAULT 'open', -- open | closed
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_pools_owner ON pools(owner_id, created_at DESC);

CREATE TABLE IF NOT EXISTS pool_contributions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pool_id      UUID NOT NULL REFERENCES pools(id) ON DELETE CASCADE,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    tx_id        UUID REFERENCES transactions(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_pool_contrib_pool ON pool_contributions(pool_id, created_at DESC);
