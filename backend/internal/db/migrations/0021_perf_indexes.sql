-- Performance indexes for the hottest read paths, which today fall back to
-- sequential scans as the tables grow.
--
-- transactions(created_at DESC): every admin report and the user's movement
-- history order/filter by created_at; without this each load seq-scans and
-- sorts the whole table.
CREATE INDEX IF NOT EXISTS idx_tx_created ON transactions(created_at DESC);

-- pool_contributions(user_id, pool_id): "vaquitas I contributed to" and the
-- per-pool contribution lookups filter by user_id with no supporting index.
CREATE INDEX IF NOT EXISTS idx_pool_contrib_user ON pool_contributions(user_id, pool_id);
