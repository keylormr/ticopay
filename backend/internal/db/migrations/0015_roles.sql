-- Server-side authorization role. The role lives only in the database and is
-- never put in the JWT or returned by /api/me; the client infers "admin" only
-- because the admin endpoints answer. Admins approve/reject merchants and set
-- commissions.
--
-- No account is auto-promoted here. The demo seed promotes the demo account in
-- development; production promotes a real account via the ADMIN_EMAIL env var
-- (applied on startup in cmd/server), so a public demo credential never becomes
-- an admin in production.
ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user';  -- user | admin
