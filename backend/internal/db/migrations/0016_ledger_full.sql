-- Make the ledger fully reconcilable against balances: external settlements
-- (service/bill payments) and currency conversions now post balanced
-- double-entry rows too. They need system counterpart accounts whose balances
-- may legitimately go negative (a net FX/clearing position), so the
-- non-negative rule moves from a blanket column CHECK to a trigger that exempts
-- system accounts.

-- Reserved system users for external clearing and FX positions (never log in).
INSERT INTO users (id, email, full_name, password_hash, kyc_status, email_verified) VALUES
  ('00000000-0000-0000-0000-0000000000c1', 'clearing@system.tuanispay', 'SYSTEM CLEARING', '', 'verified', true),
  ('00000000-0000-0000-0000-0000000000f1', 'fx@system.tuanispay',       'SYSTEM FX',       '', 'verified', true)
ON CONFLICT (id) DO NOTHING;

-- Flag system accounts; their balances may be negative (signed positions).
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS is_system BOOLEAN NOT NULL DEFAULT false;
UPDATE accounts SET is_system = true
 WHERE user_id IN ('00000000-0000-0000-0000-0000000000fe',
                   '00000000-0000-0000-0000-0000000000c1',
                   '00000000-0000-0000-0000-0000000000f1');

-- Replace the blanket non-negative CHECK with a trigger that exempts system
-- accounts. The DO block drops whatever the original column check was named.
DO $$
DECLARE c text;
BEGIN
    SELECT conname INTO c FROM pg_constraint
     WHERE conrelid = 'accounts'::regclass AND contype = 'c'
       AND pg_get_constraintdef(oid) ILIKE '%balance_cents%';
    IF c IS NOT NULL THEN
        EXECUTE format('ALTER TABLE accounts DROP CONSTRAINT %I', c);
    END IF;
END $$;

CREATE OR REPLACE FUNCTION assert_non_negative_balance() RETURNS trigger AS $$
BEGIN
    IF NEW.is_system = false AND NEW.balance_cents < 0 THEN
        RAISE EXCEPTION 'balance_cents cannot go negative for account %', NEW.id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS accounts_non_negative ON accounts;
CREATE TRIGGER accounts_non_negative
    BEFORE INSERT OR UPDATE ON accounts
    FOR EACH ROW EXECUTE FUNCTION assert_non_negative_balance();
