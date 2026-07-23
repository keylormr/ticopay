-- Double-entry ledger (audit layer) + platform fee account.
-- Balances in `accounts` stay the operational source of truth; `ledger_entries`
-- is the trigger-enforced double-entry record for wallet-to-wallet payments
-- (transfers, SINPE, cobros, vaquitas, merchant charges).

-- Reserved system user that owns the platform fee account. It never logs in
-- (empty password hash); its accounts only ever receive commission credits.
INSERT INTO users (id, email, full_name, password_hash, kyc_status, email_verified)
VALUES ('00000000-0000-0000-0000-0000000000fe', 'fees@system.tuanispay', 'SYSTEM FEES', '', 'verified', true)
ON CONFLICT (id) DO NOTHING;

-- Per-payment commission kept by the platform (0 for plain P2P).
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS fee_cents BIGINT NOT NULL DEFAULT 0 CHECK (fee_cents >= 0);

CREATE TABLE IF NOT EXISTS ledger_entries (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    account_id     UUID NOT NULL REFERENCES accounts(id),
    currency       TEXT NOT NULL,
    amount_cents   BIGINT NOT NULL,           -- signed: < 0 debit, > 0 credit
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_ledger_tx   ON ledger_entries(transaction_id);
CREATE INDEX IF NOT EXISTS idx_ledger_acct ON ledger_entries(account_id, created_at DESC);

-- Each transaction's entries must net to zero per currency. The trigger is
-- DEFERRABLE INITIALLY DEFERRED so every entry for a transaction is inserted
-- before the balance check runs, at COMMIT time.
CREATE OR REPLACE FUNCTION assert_ledger_balanced() RETURNS trigger AS $$
DECLARE
    bad INT;
BEGIN
    SELECT count(*) INTO bad FROM (
        SELECT transaction_id, currency, SUM(amount_cents) AS s
        FROM ledger_entries
        WHERE transaction_id = NEW.transaction_id
        GROUP BY transaction_id, currency
        HAVING SUM(amount_cents) <> 0
    ) q;
    IF bad > 0 THEN
        RAISE EXCEPTION 'ledger unbalanced for transaction %', NEW.transaction_id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS ledger_balanced ON ledger_entries;
CREATE CONSTRAINT TRIGGER ledger_balanced
    AFTER INSERT ON ledger_entries
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION assert_ledger_balanced();
