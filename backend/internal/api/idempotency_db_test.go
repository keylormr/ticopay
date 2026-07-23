package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests exercise the idempotency wrapper and the SINPE / pool-contribute
// money handlers against a real Postgres (skipped without TEST_DATABASE_URL).
// They pin down the Fase 0 fix: the idempotency key and the money move share one
// transaction, so a retry replays and a domain failure releases the key.

// setPhone attaches a phone number to a user so SINPE can resolve them.
func setPhone(t *testing.T, pool *pgxpool.Pool, userID, phone string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE users SET phone = $1 WHERE id = $2`, phone, userID); err != nil {
		t.Fatalf("set phone: %v", err)
	}
}

// doSinpe fires POST /sinpe as userID with an optional Idempotency-Key.
func doSinpe(t *testing.T, a *App, userID, phone string, amount float64, idemKey string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Use(withLang)
	r.Post("/sinpe", func(w http.ResponseWriter, req *http.Request) {
		ctx := context.WithValue(req.Context(), userIDKey, userID)
		a.handleSinpe(w, req.WithContext(ctx))
	})
	body := fmt.Sprintf(`{"toPhone":%q,"amount":%v}`, phone, amount)
	req := httptest.NewRequest(http.MethodPost, "/sinpe", strings.NewReader(body))
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// doContribute fires POST /pools/{id}/contribute as userID.
func doContribute(t *testing.T, a *App, userID, poolID string, amount float64, idemKey string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Use(withLang)
	r.Post("/pools/{id}/contribute", func(w http.ResponseWriter, req *http.Request) {
		ctx := context.WithValue(req.Context(), userIDKey, userID)
		a.handleContributePool(w, req.WithContext(ctx))
	})
	body := fmt.Sprintf(`{"amount":%v}`, amount)
	req := httptest.NewRequest(http.MethodPost, "/pools/"+poolID+"/contribute", strings.NewReader(body))
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestSinpeRequiresIdempotencyKey(t *testing.T) {
	pool := testDB(t)
	a := &App{pool: pool}

	sender, _ := makeUser(t, pool, "CRC", 100_000)
	recv, _ := makeUser(t, pool, "CRC", 0)
	setPhone(t, pool, recv, "88880001")

	if rec := doSinpe(t, a, sender, "88880001", 100, ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("SINPE without Idempotency-Key: status %d, want 400", rec.Code)
	}
	if got := balanceOf(t, pool, sender, "CRC"); got != 100_000 {
		t.Fatalf("no money should move without a key: sender = %d, want 100000", got)
	}
}

func TestSinpeInvalidPhoneRejected(t *testing.T) {
	pool := testDB(t)
	a := &App{pool: pool}
	sender, _ := makeUser(t, pool, "CRC", 100_000)

	if rec := doSinpe(t, a, sender, "123", 100, "k-badphone"); rec.Code != http.StatusBadRequest {
		t.Fatalf("SINPE to a 3-digit phone: status %d, want 400", rec.Code)
	}
}

func TestSinpeSelfSendRejected(t *testing.T) {
	pool := testDB(t)
	a := &App{pool: pool}
	sender, _ := makeUser(t, pool, "CRC", 100_000)
	setPhone(t, pool, sender, "88880002")

	rec := doSinpe(t, a, sender, "88880002", 100, "k-self")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("SINPE to own phone: status %d, want 400", rec.Code)
	}
	if got := balanceOf(t, pool, sender, "CRC"); got != 100_000 {
		t.Fatalf("self-send must not move money: sender = %d, want 100000", got)
	}
}

func TestSinpeIdempotentReplayMovesMoneyOnce(t *testing.T) {
	pool := testDB(t)
	a := &App{pool: pool}

	sender, _ := makeUser(t, pool, "CRC", 100_000)
	recv, _ := makeUser(t, pool, "CRC", 0)
	setPhone(t, pool, recv, "88880003")

	sent := toMinor(250, "CRC")
	first := doSinpe(t, a, sender, "88880003", 250, "k-replay")
	if first.Code != http.StatusCreated {
		t.Fatalf("first SINPE: status %d, body %s", first.Code, first.Body.String())
	}
	// A retry with the same key must replay the stored response, not move again.
	second := doSinpe(t, a, sender, "88880003", 250, "k-replay")
	if second.Code != http.StatusCreated {
		t.Fatalf("replayed SINPE: status %d, body %s", second.Code, second.Body.String())
	}
	// Compare semantically, not byte-for-byte: the stored response is JSONB, so
	// Postgres may reorder keys on read. The receipt id must match on replay.
	var b1, b2 map[string]any
	if err := json.Unmarshal(first.Body.Bytes(), &b1); err != nil {
		t.Fatalf("parse first body: %v", err)
	}
	if err := json.Unmarshal(second.Body.Bytes(), &b2); err != nil {
		t.Fatalf("parse replay body: %v", err)
	}
	if b1["comprobante"] != b2["comprobante"] || b1["comprobante"] == nil {
		t.Fatalf("replay comprobante differs: %v vs %v", b1["comprobante"], b2["comprobante"])
	}
	if got := balanceOf(t, pool, sender, "CRC"); got != 100_000-sent {
		t.Fatalf("sender charged more than once: %d, want a single charge to %d", got, 100_000-sent)
	}
	if got := balanceOf(t, pool, recv, "CRC"); got != sent {
		t.Fatalf("recipient credited more than once: %d, want %d", got, sent)
	}
}

func TestIdempotencyKeyReusedWithDifferentPayloadRejected(t *testing.T) {
	pool := testDB(t)
	a := &App{pool: pool}

	sender, _ := makeUser(t, pool, "CRC", 100_000)
	recv, _ := makeUser(t, pool, "CRC", 0)
	setPhone(t, pool, recv, "88880004")

	firstAmount := toMinor(100, "CRC")
	if rec := doSinpe(t, a, sender, "88880004", 100, "k-reuse"); rec.Code != http.StatusCreated {
		t.Fatalf("first SINPE: status %d, body %s", rec.Code, rec.Body.String())
	}
	// Same key, different amount: must be refused (422), never replayed as if it
	// were the first payment — that would swallow a distinct legitimate payment.
	rec := doSinpe(t, a, sender, "88880004", 999, "k-reuse")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("key reused with a different amount: status %d, want 422", rec.Code)
	}
	if got := balanceOf(t, pool, sender, "CRC"); got != 100_000-firstAmount {
		t.Fatalf("only the first payment should stand: sender = %d, want %d", got, 100_000-firstAmount)
	}
}

func TestDomainFailureReleasesIdempotencyKey(t *testing.T) {
	pool := testDB(t)
	a := &App{pool: pool}

	sender, _ := makeUser(t, pool, "CRC", 100) // 100 cents = 1.00 CRC
	recv, _ := makeUser(t, pool, "CRC", 0)
	setPhone(t, pool, recv, "88880005")

	// First attempt overdraws (amount 5.00 > balance 1.00): rejected AND the key
	// is released by the rollback, so the same key can be retried.
	if rec := doSinpe(t, a, sender, "88880005", 5, "k-release"); rec.Code != http.StatusBadRequest {
		t.Fatalf("overdraw SINPE: status %d, want 400", rec.Code)
	}
	// Top the sender up and retry the SAME key with the SAME payload: because the
	// failed attempt released the key, this must now execute (not 409/replay).
	// Top up off-ledger, bumping the opening offset by the same delta so the
	// reconciliation invariant (balance = opening_offset + journal) still holds.
	if _, err := pool.Exec(context.Background(),
		`UPDATE accounts SET opening_offset_cents = opening_offset_cents + 100000 - balance_cents,
		        balance_cents = 100000 WHERE user_id = $1 AND currency = 'CRC'`, sender); err != nil {
		t.Fatalf("top up: %v", err)
	}
	if rec := doSinpe(t, a, sender, "88880005", 5, "k-release"); rec.Code != http.StatusCreated {
		t.Fatalf("retry after top-up: status %d, body %s (a released key must be reusable)", rec.Code, rec.Body.String())
	}
	if got := balanceOf(t, pool, recv, "CRC"); got != 500 {
		t.Fatalf("recipient = %d, want 500 (one successful 5.00 transfer)", got)
	}
}

func TestContributePoolIdempotentReplayMovesOnce(t *testing.T) {
	pool := testDB(t)
	a := &App{pool: pool}
	ctx := context.Background()

	owner, _ := makeUser(t, pool, "CRC", 0)
	backer, _ := makeUser(t, pool, "CRC", 100_000)

	var poolID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO pools (owner_id, name, description, goal_cents, currency)
		 VALUES ($1, 'Regalo', '', 50000, 'CRC') RETURNING id`, owner,
	).Scan(&poolID); err != nil {
		t.Fatalf("create pool: %v", err)
	}

	if rec := doContribute(t, a, backer, poolID, 300, "k-pool"); rec.Code != http.StatusCreated {
		t.Fatalf("first contribute: status %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := doContribute(t, a, backer, poolID, 300, "k-pool"); rec.Code != http.StatusCreated {
		t.Fatalf("replayed contribute: status %d, body %s", rec.Code, rec.Body.String())
	}
	if got := balanceOf(t, pool, backer, "CRC"); got != 70_000 {
		t.Fatalf("backer charged more than once: %d, want a single 30000-cent contribution (70000)", got)
	}
	if got := balanceOf(t, pool, owner, "CRC"); got != 30_000 {
		t.Fatalf("owner received %d, want 30000 (contributed once)", got)
	}
	// The contribution row is recorded exactly once.
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM pool_contributions WHERE pool_id = $1 AND user_id = $2`, poolID, backer,
	).Scan(&n); err != nil {
		t.Fatalf("count contributions: %v", err)
	}
	if n != 1 {
		t.Fatalf("pool_contributions rows = %d, want 1", n)
	}
}
