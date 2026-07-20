-- Request fingerprint for idempotency keys. A key replayed with a DIFFERENT
-- payload (e.g. a client reusing an Idempotency-Key for another amount or
-- recipient) must NOT silently return the first operation's result: the server
-- compares this hash and returns 422 on a mismatch. NULL for keys created
-- before this column existed (treated as "no fingerprint recorded").
ALTER TABLE idempotency_keys ADD COLUMN IF NOT EXISTS request_hash TEXT;
