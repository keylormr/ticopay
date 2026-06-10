-- New fiat currencies (EUR, MXN). Existing users only have wallets for the
-- currencies that existed when they registered, so backfill the new ones for
-- everyone. New users get them automatically via allCurrencyCodes() on signup.
INSERT INTO accounts (user_id, currency, balance_cents)
SELECT u.id, c.code, 0
FROM users u
CROSS JOIN (VALUES ('EUR'), ('MXN')) AS c(code)
ON CONFLICT (user_id, currency) DO NOTHING;
