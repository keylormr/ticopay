package api

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// acctDrift returns balance - opening_offset - journal for one account: the
// reconciliation invariant says it must be zero.
func acctDrift(t *testing.T, pool *pgxpool.Pool, userID, currency string) int64 {
	t.Helper()
	var d int64
	if err := pool.QueryRow(context.Background(), `
		SELECT a.balance_cents - a.opening_offset_cents
		       - COALESCE((SELECT SUM(amount_cents) FROM ledger_entries WHERE account_id = a.id), 0)
		FROM accounts a WHERE a.user_id = $1 AND a.currency = $2`, userID, currency).Scan(&d); err != nil {
		t.Fatalf("acctDrift: %v", err)
	}
	return d
}

// A freshly funded account (opening offset set by makeUser) and a normal
// transfer both preserve the invariant: no drift.
func TestReconcileNoDriftOnHealthyLedger(t *testing.T) {
	pool := testDB(t)
	a := &App{pool: pool}
	ctx := context.Background()

	from, _ := makeUser(t, pool, "CRC", 100_000)
	to, toEmail := makeUser(t, pool, "CRC", 0)

	if d := acctDrift(t, pool, from, "CRC"); d != 0 {
		t.Fatalf("funded account drift = %d, want 0", d)
	}
	if _, _, err := a.transfer(ctx, from, toEmail, "CRC", 30_000, "prueba", "transfer"); err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if d := acctDrift(t, pool, from, "CRC"); d != 0 {
		t.Fatalf("sender drift after transfer = %d, want 0", d)
	}
	if d := acctDrift(t, pool, to, "CRC"); d != 0 {
		t.Fatalf("recipient drift after transfer = %d, want 0", d)
	}
}

// A balance mutated outside the ledger (no matching entry, no offset update)
// shows up as drift, and reconcileLedger reports it with the exact amount.
func TestReconcileDetectsInjectedDrift(t *testing.T) {
	pool := testDB(t)
	a := &App{pool: pool}
	ctx := context.Background()

	user, email := makeUser(t, pool, "CRC", 10_000)
	// Sneak 777 cents onto the balance with no ledger entry — a simulated bug.
	if _, err := pool.Exec(ctx,
		`UPDATE accounts SET balance_cents = balance_cents + 777 WHERE user_id = $1 AND currency = 'CRC'`, user); err != nil {
		t.Fatalf("inject: %v", err)
	}
	// Revert on cleanup so the shared integration DB stays fully reconciled for
	// any later test that might assert global ledger health.
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`UPDATE accounts SET balance_cents = balance_cents - 777 WHERE user_id = $1 AND currency = 'CRC'`, user)
	})
	if d := acctDrift(t, pool, user, "CRC"); d != 777 {
		t.Fatalf("per-account drift = %d, want 777", d)
	}

	drifts, err := a.reconcileLedger(ctx)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	// makeUser sets full_name == email (unique per test user), so match on it.
	var got int64
	var found bool
	for _, d := range drifts {
		if d.UserName == email && d.Currency == "CRC" {
			got, found = d.DriftCents, true
		}
	}
	if !found {
		t.Fatalf("reconcileLedger did not report our account; got %+v", drifts)
	}
	if got != 777 {
		t.Fatalf("reported drift = %d, want 777", got)
	}
}
