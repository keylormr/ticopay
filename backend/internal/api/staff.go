package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"ticopay/backend/internal/auth"
)

// systemEmailFilter excludes the reserved system users from back-office views.
// lower() guards against a future system user inserted with mixed case.
const systemEmailFilter = "lower(email) NOT LIKE '%@system.ticopay'"

// adminGovLock serializes admin-governance changes (promoting/demoting/disabling
// admins) via a transaction-level advisory lock, so two concurrent demotions
// can't both pass the "last admin" check and leave the platform with none.
const adminGovLock int64 = 918273

var errLastAdmin = errors.New("last admin")

// sameID compares two id strings as UUIDs when possible (the path param may
// differ in case from the canonical JWT subject), falling back to string
// equality. Prevents bypassing the self-guard with an upper-cased UUID.
func sameID(a, b string) bool {
	ua, ea := uuid.Parse(a)
	ub, eb := uuid.Parse(b)
	if ea == nil && eb == nil {
		return ua == ub
	}
	return a == b
}

type adminUserRow struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	FullName      string    `json:"fullName"`
	Phone         string    `json:"phone,omitempty"`
	Role          string    `json:"role"`
	KYCStatus     string    `json:"kycStatus"`
	EmailVerified bool      `json:"emailVerified"`
	Disabled      bool      `json:"disabled"`
	CreatedAt     time.Time `json:"createdAt"`
}

// qInt reads an int query param with a default, clamped to [lo, hi].
func qInt(r *http.Request, key string, def, lo, hi int) int {
	v, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get(key)))
	if err != nil {
		v = def
	}
	if v < lo {
		v = lo
	}
	if v > hi {
		v = hi
	}
	return v
}

// handleAdminListUsers lists real users with optional search and pagination.
func (a *App) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := qInt(r, "limit", 50, 1, 200)
	offset := qInt(r, "offset", 0, 0, 1_000_000)

	var total int
	if err := a.pool.QueryRow(r.Context(),
		`SELECT count(*) FROM users
		 WHERE `+systemEmailFilter+`
		   AND ($1 = '' OR email ILIKE '%'||$1||'%' OR full_name ILIKE '%'||$1||'%')`, q,
	).Scan(&total); err != nil {
		writeError(w, http.StatusInternalServerError, "could not count users")
		return
	}

	rows, err := a.pool.Query(r.Context(),
		`SELECT id, email, full_name, COALESCE(phone,''), role, kyc_status,
		        COALESCE(email_verified,false), disabled, created_at
		 FROM users
		 WHERE `+systemEmailFilter+`
		   AND ($1 = '' OR email ILIKE '%'||$1||'%' OR full_name ILIKE '%'||$1||'%')
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, q, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load users")
		return
	}
	defer rows.Close()

	list := make([]adminUserRow, 0, limit)
	for rows.Next() {
		var u adminUserRow
		if err := rows.Scan(&u.ID, &u.Email, &u.FullName, &u.Phone, &u.Role, &u.KYCStatus,
			&u.EmailVerified, &u.Disabled, &u.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "could not read users")
			return
		}
		list = append(list, u)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"users": list, "total": total, "limit": limit, "offset": offset, "roles": assignableRoles(),
	})
}

// handleAdminCreateUser creates a (typically staff) account with a role. Staff
// accounts are created pre-verified; the admin sets the initial password.
func (a *App) handleAdminCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		FullName string `json:"fullName"`
		Phone    string `json:"phone"`
		Role     string `json:"role"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.FullName = strings.TrimSpace(req.FullName)
	req.Role = strings.TrimSpace(req.Role)
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		writeError(w, http.StatusBadRequest, "ingresá un correo válido")
		return
	}
	if req.FullName == "" {
		writeError(w, http.StatusBadRequest, "el nombre es obligatorio")
		return
	}
	if !isValidRole(req.Role) {
		writeError(w, http.StatusBadRequest, "rol no válido")
		return
	}
	if msg := validateStaffPassword(req.Password); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if strings.HasSuffix(req.Email, "@system.ticopay") {
		writeError(w, http.StatusBadRequest, "correo reservado")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not hash password")
		return
	}

	ctx := r.Context()
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer tx.Rollback(ctx)

	var u adminUserRow
	var phone *string
	if p := strings.TrimSpace(req.Phone); p != "" {
		phone = &p
	}
	err = tx.QueryRow(ctx,
		`INSERT INTO users (email, phone, full_name, password_hash, role, kyc_status, email_verified)
		 VALUES ($1, $2, $3, $4, $5, 'verified', true)
		 RETURNING id, email, full_name, COALESCE(phone,''), role, kyc_status, COALESCE(email_verified,false), disabled, created_at`,
		req.Email, phone, req.FullName, hash, req.Role,
	).Scan(&u.ID, &u.Email, &u.FullName, &u.Phone, &u.Role, &u.KYCStatus, &u.EmailVerified, &u.Disabled, &u.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "ya existe un usuario con ese correo o teléfono")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not create user")
		return
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO accounts (user_id, currency, balance_cents)
		 SELECT $1, code, 0 FROM unnest($2::text[]) AS code`, u.ID, allCurrencyCodes(),
	); err != nil {
		writeError(w, http.StatusInternalServerError, "could not create accounts")
		return
	}
	if err := auditTx(ctx, tx, userID(r), auditUserCreate, u.ID,
		map[string]any{"email": u.Email, "role": u.Role}); err != nil {
		writeError(w, http.StatusInternalServerError, "could not create user")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "could not create user")
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

// handleAdminSetRole changes a user's role. An admin cannot strip their own
// admin role (avoids locking the platform out of administration), and system
// users can't be touched.
func (a *App) handleAdminSetRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Role = strings.TrimSpace(req.Role)
	if !isValidRole(req.Role) {
		writeError(w, http.StatusBadRequest, "rol no válido")
		return
	}
	if sameID(id, userID(r)) && req.Role != roleAdmin {
		writeError(w, http.StatusBadRequest, "no podés quitarte tu propio rol de administrador")
		return
	}
	ctx := r.Context()
	err := a.inTx(ctx, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, adminGovLock); e != nil {
			return errTransferOther
		}
		var curRole string
		e := tx.QueryRow(ctx, `SELECT role FROM users WHERE id = $1 AND `+systemEmailFilter+` FOR UPDATE`, id).Scan(&curRole)
		if errors.Is(e, pgx.ErrNoRows) {
			return errReqNotFound
		}
		if e != nil {
			return errTransferOther
		}
		// Don't demote the last active admin out of administration.
		if curRole == roleAdmin && req.Role != roleAdmin {
			var others int
			if e := tx.QueryRow(ctx,
				`SELECT count(*) FROM users WHERE role = 'admin' AND NOT disabled AND id <> $1 AND `+systemEmailFilter, id,
			).Scan(&others); e != nil {
				return errTransferOther
			}
			if others == 0 {
				return errLastAdmin
			}
		}
		if _, e := tx.Exec(ctx, `UPDATE users SET role = $2 WHERE id = $1 AND `+systemEmailFilter, id, req.Role); e != nil {
			return errTransferOther
		}
		return auditTx(ctx, tx, userID(r), auditUserSetRole, id,
			map[string]any{"role": req.Role, "previous": curRole})
	})
	writeAdminResult(w, err, map[string]any{"role": req.Role})
}

// writeAdminResult maps the governance-tx outcome to an HTTP response.
func writeAdminResult(w http.ResponseWriter, err error, ok map[string]any) {
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, ok)
	case errors.Is(err, errReqNotFound):
		writeError(w, http.StatusNotFound, "usuario no encontrado")
	case errors.Is(err, errLastAdmin):
		writeError(w, http.StatusConflict, "no podés dejar la plataforma sin administradores")
	default:
		writeError(w, http.StatusInternalServerError, "no se pudo completar la operación")
	}
}

// handleAdminSetStatus enables/disables a user. Disabling also bumps
// token_version, revoking the user's active sessions immediately. An admin
// can't disable their own account.
func (a *App) handleAdminSetStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Disabled bool `json:"disabled"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	if sameID(id, userID(r)) && req.Disabled {
		writeError(w, http.StatusBadRequest, "no podés desactivar tu propia cuenta")
		return
	}
	bump := 0
	if req.Disabled {
		bump = 1
	}
	ctx := r.Context()
	err := a.inTx(ctx, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, adminGovLock); e != nil {
			return errTransferOther
		}
		var curRole string
		e := tx.QueryRow(ctx, `SELECT role FROM users WHERE id = $1 AND `+systemEmailFilter+` FOR UPDATE`, id).Scan(&curRole)
		if errors.Is(e, pgx.ErrNoRows) {
			return errReqNotFound
		}
		if e != nil {
			return errTransferOther
		}
		// Don't disable the last active admin.
		if req.Disabled && curRole == roleAdmin {
			var others int
			if e := tx.QueryRow(ctx,
				`SELECT count(*) FROM users WHERE role = 'admin' AND NOT disabled AND id <> $1 AND `+systemEmailFilter, id,
			).Scan(&others); e != nil {
				return errTransferOther
			}
			if others == 0 {
				return errLastAdmin
			}
		}
		if _, e := tx.Exec(ctx,
			`UPDATE users SET disabled = $2, token_version = token_version + $3 WHERE id = $1 AND `+systemEmailFilter,
			id, req.Disabled, bump); e != nil {
			return errTransferOther
		}
		return auditTx(ctx, tx, userID(r), auditUserSetStatus, id,
			map[string]any{"disabled": req.Disabled})
	})
	writeAdminResult(w, err, map[string]any{"disabled": req.Disabled})
}
