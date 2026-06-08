-- Add crypto wallets (BTC, ETH, USDT) alongside fiat (CRC, USD).
-- Balances are integer minor units per currency's own precision
-- (BTC/ETH = 8 decimals, USDT = 2 decimals).

INSERT INTO accounts (user_id, currency, balance_cents)
SELECT u.id, c.code, 0
FROM users u
CROSS JOIN (VALUES ('BTC'), ('ETH'), ('USDT')) AS c(code)
ON CONFLICT (user_id, currency) DO NOTHING;

-- Demo crypto balances for María so the wallet isn't empty.
UPDATE accounts SET balance_cents = 500000
WHERE currency = 'BTC' AND user_id = (SELECT id FROM users WHERE email = 'maria@ticopay.cr');  -- 0.005 BTC
UPDATE accounts SET balance_cents = 10000000
WHERE currency = 'ETH' AND user_id = (SELECT id FROM users WHERE email = 'maria@ticopay.cr');  -- 0.1 ETH
UPDATE accounts SET balance_cents = 10000
WHERE currency = 'USDT' AND user_id = (SELECT id FROM users WHERE email = 'maria@ticopay.cr'); -- 100.00 USDT
