package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestSanitizeCSVCell is a pure unit test (no DB) for the formula-injection guard.
func TestSanitizeCSVCell(t *testing.T) {
	cases := map[string]string{
		"":            "",
		"hola":        "hola",
		"normal text": "normal text",
		"=1+1":        "'=1+1",
		"+1":          "'+1",
		"-1":          "'-1",
		"@SUM(A1)":    "'@SUM(A1)",
		"\tx":         "'\tx",
	}
	for in, want := range cases {
		if got := sanitizeCSVCell(in); got != want {
			t.Errorf("sanitizeCSVCell(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStaffCreateRoleAndStatus(t *testing.T) {
	pool := testDB(t)
	a := &App{pool: pool}
	ctx := context.Background()

	admin, _ := makeUser(t, pool, "CRC", 0)
	if _, err := pool.Exec(ctx, `UPDATE users SET role = 'admin' WHERE id = $1`, admin); err != nil {
		t.Fatalf("promote admin: %v", err)
	}

	// Create a support staff account.
	rec := fireAs(t, admin, http.MethodPost, "/admin/users", "/admin/users",
		`{"email":"sup@ticopay.cr","fullName":"Soporte Uno","role":"support","password":"password123"}`,
		a.handleAdminCreateUser, a.requirePerm(permUsersManage))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create staff: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID   string `json:"id"`
		Role string `json:"role"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.Role != "support" {
		t.Fatalf("role = %q, want support", created.Role)
	}

	// Change the staff role to analyst.
	rec = fireAs(t, admin, http.MethodPost, "/admin/users/{id}/role", "/admin/users/"+created.ID+"/role",
		`{"role":"analyst"}`, a.handleAdminSetRole, a.requirePerm(permUsersManage))
	if rec.Code != http.StatusOK {
		t.Fatalf("set role: %d %s", rec.Code, rec.Body.String())
	}
	var role string
	_ = pool.QueryRow(ctx, `SELECT role FROM users WHERE id = $1`, created.ID).Scan(&role)
	if role != "analyst" {
		t.Fatalf("persisted role = %q, want analyst", role)
	}

	// An invalid role is rejected.
	rec = fireAs(t, admin, http.MethodPost, "/admin/users/{id}/role", "/admin/users/"+created.ID+"/role",
		`{"role":"superuser"}`, a.handleAdminSetRole, a.requirePerm(permUsersManage))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid role: %d, want 400", rec.Code)
	}

	// Self-lockout guard: an admin can't strip their own admin role.
	rec = fireAs(t, admin, http.MethodPost, "/admin/users/{id}/role", "/admin/users/"+admin+"/role",
		`{"role":"user"}`, a.handleAdminSetRole, a.requirePerm(permUsersManage))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("self-demotion: %d, want 400", rec.Code)
	}

	// Disabling a user bumps token_version (revoking active sessions).
	var tvBefore int
	_ = pool.QueryRow(ctx, `SELECT token_version FROM users WHERE id = $1`, created.ID).Scan(&tvBefore)
	rec = fireAs(t, admin, http.MethodPost, "/admin/users/{id}/status", "/admin/users/"+created.ID+"/status",
		`{"disabled":true}`, a.handleAdminSetStatus, a.requirePerm(permUsersManage))
	if rec.Code != http.StatusOK {
		t.Fatalf("disable: %d %s", rec.Code, rec.Body.String())
	}
	var disabled bool
	var tvAfter int
	_ = pool.QueryRow(ctx, `SELECT disabled, token_version FROM users WHERE id = $1`, created.ID).Scan(&disabled, &tvAfter)
	if !disabled {
		t.Fatal("user not disabled")
	}
	if tvAfter != tvBefore+1 {
		t.Fatalf("token_version = %d, want %d (bumped on disable)", tvAfter, tvBefore+1)
	}
}

// TestSelfDemoteBlockedUppercaseUUID verifies the self-guard can't be bypassed
// by passing the actor's own id upper-cased in the path.
func TestSelfDemoteBlockedUppercaseUUID(t *testing.T) {
	pool := testDB(t)
	a := &App{pool: pool}
	ctx := context.Background()

	admin, _ := makeUser(t, pool, "CRC", 0)
	if _, err := pool.Exec(ctx, `UPDATE users SET role = 'admin' WHERE id = $1`, admin); err != nil {
		t.Fatalf("promote admin: %v", err)
	}
	upper := strings.ToUpper(admin)
	rec := fireAs(t, admin, http.MethodPost, "/admin/users/{id}/role", "/admin/users/"+upper+"/role",
		`{"role":"user"}`, a.handleAdminSetRole, a.requirePerm(permUsersManage))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("self-demotion via upper-cased UUID: %d, want 400 (self-guard)", rec.Code)
	}
	// Role must be unchanged.
	var role string
	_ = pool.QueryRow(ctx, `SELECT role FROM users WHERE id = $1`, admin).Scan(&role)
	if role != "admin" {
		t.Fatalf("role changed to %q despite self-guard", role)
	}
}
