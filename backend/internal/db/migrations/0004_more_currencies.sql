-- Expand the catalog to a Binance-like set of crypto wallets.
-- Back-fill wallets for all existing users (new currencies only; 0003 added BTC/ETH/USDT).

INSERT INTO accounts (user_id, currency, balance_cents)
SELECT u.id, c.code, 0
FROM users u
CROSS JOIN (VALUES
  ('USDC'), ('BNB'), ('SOL'), ('XRP'), ('ADA'),
  ('DOGE'), ('TRX'), ('DOT'), ('LTC'), ('LINK'), ('AVAX'), ('MATIC')
) AS c(code)
ON CONFLICT (user_id, currency) DO NOTHING;

-- A little extra demo variety for María.
UPDATE accounts SET balance_cents = 150000000
WHERE currency = 'SOL' AND user_id = (SELECT id FROM users WHERE email = 'maria@ticopay.cr');  -- 1.5 SOL
UPDATE accounts SET balance_cents = 30000000
WHERE currency = 'BNB' AND user_id = (SELECT id FROM users WHERE email = 'maria@ticopay.cr');  -- 0.3 BNB
