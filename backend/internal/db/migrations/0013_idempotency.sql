-- End-to-end idempotency for money POSTs without a natural unique key
-- (send, SINPE, service payment, pool contribution). The key is the
-- client-supplied Idempotency-Key namespaced by user; the stored response is
-- replayed verbatim on retries so a network re-send never moves money twice.
CREATE TABLE IF NOT EXISTS idempotency_keys (
    key         TEXT PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status_code INT,
    response    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_idem_user ON idempotency_keys(user_id, created_at DESC);
