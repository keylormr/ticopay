package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// fireAs invokes a single handler as the given user, injecting userID into the
// request context (like the real requireAuth would) and optionally running
// extra middleware (e.g. requireAdmin) in between. DB-gated like the rest.
func fireAs(t *testing.T, userID, method, pattern, target, jsonBody string, h http.HandlerFunc, mw ...func(http.Handler) http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Use(withLang)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(context.WithValue(req.Context(), userIDKey, userID)))
		})
	})
	for _, m := range mw {
		r.Use(m)
	}
	r.Method(method, pattern, h)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, strings.NewReader(jsonBody))
	r.ServeHTTP(rec, req)
	return rec
}

func TestMerchantFlowEndToEnd(t *testing.T) {
	pool := testDB(t)
	a := &App{pool: pool}
	ctx := context.Background()

	owner, _ := makeUser(t, pool, "CRC", 0)
	payer, _ := makeUser(t, pool, "CRC", 200_000)
	admin, _ := makeUser(t, pool, "CRC", 0)
	if _, err := pool.Exec(ctx, `UPDATE users SET role = 'admin' WHERE id = $1`, admin); err != nil {
		t.Fatalf("promote admin: %v", err)
	}

	// 1. Owner registers a merchant -> starts pending.
	rec := fireAs(t, owner, http.MethodPost, "/merchants", "/merchants",
		`{"name":"Soda La Esquina","category":"restaurante"}`, a.handleCreateMerchant)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create merchant: %d %s", rec.Code, rec.Body.String())
	}
	var m struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode merchant: %v", err)
	}
	if m.Status != "pending" {
		t.Fatalf("new merchant status = %q, want pending", m.Status)
	}

	// 2. Charging while pending is forbidden.
	rec = fireAs(t, owner, http.MethodPost, "/merchants/{id}/charge", "/merchants/"+m.ID+"/charge",
		`{"amount":100000,"currency":"CRC"}`, a.handleCreateMerchantCharge)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("charge while pending: %d, want 403", rec.Code)
	}

	// 3. Admin verifies the merchant.
	rec = fireAs(t, admin, http.MethodPost, "/admin/merchants/{id}/verify", "/admin/merchants/"+m.ID+"/verify",
		`{}`, a.handleAdminVerifyMerchant, a.requirePerm(permMerchantsVerify))
	if rec.Code != http.StatusOK {
		t.Fatalf("admin verify: %d %s", rec.Code, rec.Body.String())
	}

	// 4. A non-owner cannot charge this merchant (ownership gate -> 404).
	rec = fireAs(t, payer, http.MethodPost, "/merchants/{id}/charge", "/merchants/"+m.ID+"/charge",
		`{"amount":100000,"currency":"CRC"}`, a.handleCreateMerchantCharge)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("charge by non-owner: %d, want 404", rec.Code)
	}

	// 5. Owner now charges -> creates a merchant cobro.
	rec = fireAs(t, owner, http.MethodPost, "/merchants/{id}/charge", "/merchants/"+m.ID+"/charge",
		`{"amount":100000,"currency":"CRC"}`, a.handleCreateMerchantCharge)
	if rec.Code != http.StatusCreated {
		t.Fatalf("charge verified: %d %s", rec.Code, rec.Body.String())
	}
	var ch struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ch); err != nil {
		t.Fatalf("decode charge: %v", err)
	}

	// 6. Payer pays the charge -> commission split (100000 @ 50 bps = 500 fee).
	if rec := payCobro(t, a, payer, ch.ID); rec.Code != http.StatusOK {
		t.Fatalf("pay merchant charge: %d %s", rec.Code, rec.Body.String())
	}
	if got := balanceOf(t, pool, payer, "CRC"); got != 100_000 {
		t.Fatalf("payer balance = %d, want 100000", got)
	}
	if got := balanceOf(t, pool, owner, "CRC"); got != 99_500 {
		t.Fatalf("merchant net = %d, want 99500", got)
	}
}

func TestRequirePermBlocksNonBackoffice(t *testing.T) {
	pool := testDB(t)
	a := &App{pool: pool}
	ctx := context.Background()

	user, _ := makeUser(t, pool, "CRC", 0) // default role 'user' -> no back-office
	admin, _ := makeUser(t, pool, "CRC", 0)
	if _, err := pool.Exec(ctx, `UPDATE users SET role = 'admin' WHERE id = $1`, admin); err != nil {
		t.Fatalf("promote admin: %v", err)
	}

	if rec := fireAs(t, user, http.MethodGet, "/admin/whoami", "/admin/whoami", "",
		a.handleAdminWhoami, a.requirePerm(permBackoffice)); rec.Code != http.StatusForbidden {
		t.Fatalf("non-back-office to /admin/whoami: %d, want 403", rec.Code)
	}
	if rec := fireAs(t, admin, http.MethodGet, "/admin/whoami", "/admin/whoami", "",
		a.handleAdminWhoami, a.requirePerm(permBackoffice)); rec.Code != http.StatusOK {
		t.Fatalf("admin to /admin/whoami: %d, want 200", rec.Code)
	}
}

func TestSupportCannotManageUsersOrCommission(t *testing.T) {
	pool := testDB(t)
	a := &App{pool: pool}
	ctx := context.Background()

	support, _ := makeUser(t, pool, "CRC", 0)
	if _, err := pool.Exec(ctx, `UPDATE users SET role = 'support' WHERE id = $1`, support); err != nil {
		t.Fatalf("set support: %v", err)
	}
	// Support has back-office + reports + verify, but NOT users.manage.
	if rec := fireAs(t, support, http.MethodPost, "/admin/users", "/admin/users",
		`{"email":"x@y.cr","fullName":"X","role":"support","password":"password123"}`,
		a.handleAdminCreateUser, a.requirePerm(permUsersManage)); rec.Code != http.StatusForbidden {
		t.Fatalf("support creating users: %d, want 403", rec.Code)
	}
	// But support CAN reach a reports endpoint (reports.view).
	if rec := fireAs(t, support, http.MethodGet, "/admin/reports/overview", "/admin/reports/overview", "",
		a.handleReportOverview, a.requirePerm(permReportsView)); rec.Code != http.StatusOK {
		t.Fatalf("support viewing reports: %d, want 200", rec.Code)
	}
}
