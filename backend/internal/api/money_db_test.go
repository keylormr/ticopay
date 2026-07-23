package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tuanispay/backend/internal/db"
)

// These tests exercise the real money path against a Postgres database. They
// are skipped unless TEST_DATABASE_URL points at a disposable database (CI sets
// it via a postgres service); plain `go test ./...` runs the pure-logic tests
// only. This is the safety net KiramoPay learned it needed: the double-entry
// trigger, idempotency and commission split only show their teeth against a
// real DB.

var seq int64

func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping DB integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// makeUser creates a fresh user with one account of the given currency/balance.
func makeUser(t *testing.T, pool *pgxpool.Pool, currency string, balance int64) (userID, email string) {
	t.Helper()
	ctx := context.Background()
	email = fmt.Sprintf("u%d@test.local", atomic.AddInt64(&seq, 1))
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, full_name, password_hash, kyc_status, email_verified)
		 VALUES ($1, $1, '', 'verified', true) RETURNING id`, email,
	).Scan(&userID); err != nil {
		t.Fatalf("create user: %v", err)
	}
	// Funding an account directly (outside the ledger) sets its opening offset,
	// mirroring the seed path, so the reconciliation invariant holds.
	if _, err := pool.Exec(ctx,
		`INSERT INTO accounts (user_id, currency, balance_cents, opening_offset_cents) VALUES ($1, $2, $3, $3)`,
		userID, currency, balance); err != nil {
		t.Fatalf("create account: %v", err)
	}
	return userID, email
}

func balanceOf(t *testing.T, pool *pgxpool.Pool, userID, currency string) int64 {
	t.Helper()
	var b int64
	if err := pool.QueryRow(context.Background(),
		`SELECT balance_cents FROM accounts WHERE user_id = $1 AND currency = $2`, userID, currency,
	).Scan(&b); err != nil {
		t.Fatalf("balance: %v", err)
	}
	return b
}

func ledgerSum(t *testing.T, pool *pgxpool.Pool, txID string) int64 {
	t.Helper()
	var s int64
	if err := pool.QueryRow(context.Background(),
		`SELECT COALESCE(SUM(amount_cents), 0) FROM ledger_entries WHERE transaction_id = $1`, txID,
	).Scan(&s); err != nil {
		t.Fatalf("ledger sum: %v", err)
	}
	return s
}

func TestTransferMovesMoneyAndLedgerBalances(t *testing.T) {
	pool := testDB(t)
	a := &App{pool: pool}
	ctx := context.Background()

	from, _ := makeUser(t, pool, "CRC", 100_000)
	to, toEmail := makeUser(t, pool, "CRC", 0)

	txID, newBal, err := a.transfer(ctx, from, toEmail, "CRC", 30_000, "prueba", "transfer")
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if newBal != 70_000 {
		t.Fatalf("sender balance = %d, want 70000", newBal)
	}
	if got := balanceOf(t, pool, from, "CRC"); got != 70_000 {
		t.Fatalf("from account = %d, want 70000", got)
	}
	if got := balanceOf(t, pool, to, "CRC"); got != 30_000 {
		t.Fatalf("to account = %d, want 30000", got)
	}
	if s := ledgerSum(t, pool, txID); s != 0 {
		t.Fatalf("ledger entries for tx do not net to zero: sum = %d", s)
	}
}

// feesBalance returns the SYSTEM:FEES balance for a currency (0 if the account
// doesn't exist yet). SYSTEM:FEES is shared across tests, so assert deltas.
func feesBalance(t *testing.T, pool *pgxpool.Pool, currency string) int64 {
	t.Helper()
	var b int64
	if err := pool.QueryRow(context.Background(),
		`SELECT COALESCE(SUM(balance_cents), 0) FROM accounts WHERE user_id = $1 AND currency = $2`,
		sysFeesUserID, currency).Scan(&b); err != nil {
		t.Fatalf("fees balance: %v", err)
	}
	return b
}

func TestInsufficientFundsRollsBack(t *testing.T) {
	pool := testDB(t)
	a := &App{pool: pool}
	ctx := context.Background()

	from, _ := makeUser(t, pool, "CRC", 100)
	to, toEmail := makeUser(t, pool, "CRC", 0)

	if _, _, err := a.transfer(ctx, from, toEmail, "CRC", 500, "prueba", "transfer"); err == nil {
		t.Fatal("expected insufficient-funds error, got nil")
	}
	if got := balanceOf(t, pool, from, "CRC"); got != 100 {
		t.Fatalf("from account changed on failed transfer: %d, want 100", got)
	}
	if got := balanceOf(t, pool, to, "CRC"); got != 0 {
		t.Fatalf("to account changed on failed transfer: %d, want 0", got)
	}
}

// payCobro fires POST /requests/{id}/pay as the given payer.
func payCobro(t *testing.T, a *App, payerID, reqID string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Use(withLang)
	r.Post("/requests/{id}/pay", func(w http.ResponseWriter, req *http.Request) {
		ctx := context.WithValue(req.Context(), userIDKey, payerID)
		a.handlePayRequest(w, req.WithContext(ctx))
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/requests/"+reqID+"/pay", strings.NewReader(`{"amount":0}`))
	r.ServeHTTP(rec, req)
	return rec
}

func TestPayRequestIsIdempotentNoDoublePay(t *testing.T) {
	pool := testDB(t)
	a := &App{pool: pool}
	ctx := context.Background()

	requester, _ := makeUser(t, pool, "CRC", 0)
	payer, _ := makeUser(t, pool, "CRC", 100_000)

	var reqID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO payment_requests (requester_id, amount_cents, currency, description)
		 VALUES ($1, 20000, 'CRC', 'cobro') RETURNING id`, requester,
	).Scan(&reqID); err != nil {
		t.Fatalf("create cobro: %v", err)
	}

	if rec := payCobro(t, a, payer, reqID); rec.Code != http.StatusOK {
		t.Fatalf("first pay: status %d, body %s", rec.Code, rec.Body.String())
	}
	// A retry of the same cobro by the same payer must NOT move money again.
	if rec := payCobro(t, a, payer, reqID); rec.Code != http.StatusOK {
		t.Fatalf("retry pay: status %d, body %s", rec.Code, rec.Body.String())
	}

	if got := balanceOf(t, pool, payer, "CRC"); got != 80_000 {
		t.Fatalf("payer charged %d total, want a single 20000 charge (balance 80000)", got)
	}
	if got := balanceOf(t, pool, requester, "CRC"); got != 20_000 {
		t.Fatalf("requester received %d, want 20000 (paid once)", got)
	}
}

func TestMerchantCommissionSplit(t *testing.T) {
	pool := testDB(t)
	a := &App{pool: pool}
	ctx := context.Background()

	owner, _ := makeUser(t, pool, "CRC", 0)
	payer, _ := makeUser(t, pool, "CRC", 200_000)

	var merchantID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO merchants (owner_id, name, status, commission_bps)
		 VALUES ($1, 'Soda', 'verified', 50) RETURNING id`, owner,
	).Scan(&merchantID); err != nil {
		t.Fatalf("create merchant: %v", err)
	}
	var reqID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO payment_requests (requester_id, merchant_id, amount_cents, currency, description)
		 VALUES ($1, $2, 100000, 'CRC', 'venta') RETURNING id`, owner, merchantID,
	).Scan(&reqID); err != nil {
		t.Fatalf("create merchant charge: %v", err)
	}

	feesBefore := feesBalance(t, pool, "CRC")
	if rec := payCobro(t, a, payer, reqID); rec.Code != http.StatusOK {
		t.Fatalf("merchant pay: status %d, body %s", rec.Code, rec.Body.String())
	}

	// Payer pays the full 100000; merchant receives net 99500; fees account +500.
	if got := balanceOf(t, pool, payer, "CRC"); got != 100_000 {
		t.Fatalf("payer balance = %d, want 100000", got)
	}
	if got := balanceOf(t, pool, owner, "CRC"); got != 99_500 {
		t.Fatalf("merchant net = %d, want 99500", got)
	}
	// SYSTEM:FEES is shared across tests, so assert the delta from this payment.
	if got := feesBalance(t, pool, "CRC") - feesBefore; got != 500 {
		t.Fatalf("SYSTEM:FEES delta = %d, want 500", got)
	}
}

func TestLedgerTriggerRejectsUnbalanced(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()

	// A real account + transaction to attach a (deliberately unbalanced) entry.
	owner, _ := makeUser(t, pool, "CRC", 1000)
	var accID string
	if err := pool.QueryRow(ctx,
		`SELECT id FROM accounts WHERE user_id = $1 AND currency = 'CRC'`, owner).Scan(&accID); err != nil {
		t.Fatalf("acct: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)
	var txID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO transactions (from_account_id, amount_cents, currency, kind)
		 VALUES ($1, 100, 'CRC', 'transfer') RETURNING id`, accID).Scan(&txID); err != nil {
		t.Fatalf("insert tx: %v", err)
	}
	// One-sided entry: should make COMMIT fail via the deferred balance trigger.
	if _, err := tx.Exec(ctx,
		`INSERT INTO ledger_entries (transaction_id, account_id, currency, amount_cents) VALUES ($1, $2, 'CRC', 100)`,
		txID, accID); err != nil {
		t.Fatalf("insert entry: %v", err)
	}
	if err := tx.Commit(ctx); err == nil {
		t.Fatal("expected commit to fail on unbalanced ledger, got nil")
	}
}

func ledgerSumCur(t *testing.T, pool *pgxpool.Pool, txID, currency string) int64 {
	t.Helper()
	var s int64
	if err := pool.QueryRow(context.Background(),
		`SELECT COALESCE(SUM(amount_cents), 0) FROM ledger_entries WHERE transaction_id = $1 AND currency = $2`,
		txID, currency).Scan(&s); err != nil {
		t.Fatalf("ledger sum cur: %v", err)
	}
	return s
}

func ledgerForUser(t *testing.T, pool *pgxpool.Pool, txID, userID string) int64 {
	t.Helper()
	var s int64
	if err := pool.QueryRow(context.Background(),
		`SELECT COALESCE(SUM(le.amount_cents), 0) FROM ledger_entries le
		 JOIN accounts ac ON ac.id = le.account_id
		 WHERE le.transaction_id = $1 AND ac.user_id = $2`, txID, userID).Scan(&s); err != nil {
		t.Fatalf("ledger for user: %v", err)
	}
	return s
}

func TestPayOutIsLedgered(t *testing.T) {
	pool := testDB(t)
	a := &App{pool: pool}
	ctx := context.Background()

	user, _ := makeUser(t, pool, "CRC", 50_000)
	txID, newBal, err := a.payOut(ctx, user, "CRC", 20_000, "ICE", "service")
	if err != nil {
		t.Fatalf("payOut: %v", err)
	}
	if newBal != 30_000 || balanceOf(t, pool, user, "CRC") != 30_000 {
		t.Fatalf("payer balance = %d, want 30000", balanceOf(t, pool, user, "CRC"))
	}
	// The external leg is credited to SYSTEM:CLEARING and the entries balance.
	if got := ledgerForUser(t, pool, txID, sysClearingUserID); got != 20_000 {
		t.Fatalf("SYSTEM:CLEARING ledger entry = %d, want 20000", got)
	}
	if s := ledgerSum(t, pool, txID); s != 0 {
		t.Fatalf("payOut ledger not balanced: sum = %d", s)
	}
}

func TestConvertIsLedgeredAndFXCanGoNegative(t *testing.T) {
	pool := testDB(t)
	a := &App{pool: pool}
	ctx := context.Background()

	user, _ := makeUser(t, pool, "CRC", 100_000)
	if _, err := pool.Exec(ctx, `INSERT INTO accounts (user_id, currency, balance_cents) VALUES ($1, 'USD', 0)`, user); err != nil {
		t.Fatalf("usd account: %v", err)
	}

	var txID string
	if err := a.inTx(ctx, func(tx pgx.Tx) error {
		id, e := a.postConversionTx(ctx, tx, user, "CRC", "USD", 60_000, 1_000, "conv")
		txID = id
		return e
	}); err != nil {
		t.Fatalf("convert: %v", err)
	}

	if got := balanceOf(t, pool, user, "CRC"); got != 40_000 {
		t.Fatalf("user CRC = %d, want 40000", got)
	}
	if got := balanceOf(t, pool, user, "USD"); got != 1_000 {
		t.Fatalf("user USD = %d, want 1000", got)
	}
	// The FX desk holds a signed position: long CRC, short (negative) USD.
	if got := balanceOf(t, pool, sysFXUserID, "USD"); got >= 0 {
		t.Fatalf("SYSTEM:FX USD = %d, want negative (system accounts may go negative)", got)
	}
	// Each currency nets to zero across the conversion's ledger entries.
	if s := ledgerSumCur(t, pool, txID, "CRC"); s != 0 {
		t.Fatalf("conversion CRC leg not balanced: sum = %d", s)
	}
	if s := ledgerSumCur(t, pool, txID, "USD"); s != 0 {
		t.Fatalf("conversion USD leg not balanced: sum = %d", s)
	}
}

func TestNonSystemAccountCannotGoNegative(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()

	user, _ := makeUser(t, pool, "CRC", 100)
	var accID string
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts WHERE user_id = $1 AND currency = 'CRC'`, user).Scan(&accID); err != nil {
		t.Fatalf("acct: %v", err)
	}
	// The non-negative trigger must still protect ordinary (non-system) accounts.
	if _, err := pool.Exec(ctx, `UPDATE accounts SET balance_cents = -50 WHERE id = $1`, accID); err == nil {
		t.Fatal("expected the non-negative trigger to reject a negative balance on a user account")
	}
}
