-- Reconciliation baseline for the ledger.
--
-- The double-entry journal (ledger_entries) only records wallet-to-wallet
-- movements, NOT the initial funding of an account (seed balances, demo grants,
-- any pre-ledger deposit). So today balance_cents != SUM(journal) and drift is
-- undetectable. opening_offset_cents captures that off-journal portion, making
-- the invariant exact:
--
--     balance_cents = opening_offset_cents + SUM(ledger_entries for the account)
--
-- Every money path already updates the balance and inserts balanced ledger
-- entries in the same transaction, so the invariant is preserved going forward.
-- Any later divergence means a balance was mutated without a matching entry — a
-- bug, surfaced by the reconciliation worker as the tuanispay_ledger_drift_cents
-- metric and a high-severity log.
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS opening_offset_cents BIGINT NOT NULL DEFAULT 0;

-- One-time backfill: set each existing account's offset to (balance - journal
-- sum) so the invariant holds exactly at this baseline (drift = 0 for every
-- account now). The `WHERE opening_offset_cents = 0` guard makes this a
-- backfill of the freshly-added column only — so it can never re-run after
-- traffic and silently absorb real drift into the offset (schema_migrations
-- already prevents re-application; this is belt-and-suspenders).
UPDATE accounts a
SET opening_offset_cents = a.balance_cents - COALESCE(
    (SELECT SUM(le.amount_cents) FROM ledger_entries le WHERE le.account_id = a.id), 0)
WHERE a.opening_offset_cents = 0;
