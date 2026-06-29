package api

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"ticopay/backend/internal/models"
)

// requireAdmin gates admin-only routes. The role is read from the database on
// every request and never travels in the JWT or /api/me: the client can only
// infer "admin" because these endpoints answer. Must run after requireAuth.
func (a *App) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var role string
		err := a.pool.QueryRow(r.Context(), `SELECT role FROM users WHERE id = $1`, userID(r)).Scan(&role)
		if err != nil || role != "admin" {
			writeError(w, http.StatusForbidden, "no autorizado")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleAdminWhoami answers 200 only for admins (the route is behind
// requireAdmin), letting the client decide whether to show the admin panel.
func (a *App) handleAdminWhoami(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"admin": true})
}

// handleAdminListMerchants lists merchants (pending first) for review.
func (a *App) handleAdminListMerchants(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(),
		`SELECT m.id, m.name, m.category, m.legal_name, COALESCE(m.id_type,''), COALESCE(m.id_number,''),
		        m.status, m.reject_reason, m.commission_bps, u.email, m.created_at
		 FROM merchants m JOIN users u ON u.id = m.owner_id
		 ORDER BY (m.status = 'pending') DESC, m.created_at DESC LIMIT 200`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load merchants")
		return
	}
	defer rows.Close()

	list := make([]models.Merchant, 0)
	for rows.Next() {
		var m models.Merchant
		if err := rows.Scan(&m.ID, &m.Name, &m.Category, &m.LegalName, &m.IDType, &m.IDNumber,
			&m.Status, &m.RejectReason, &m.CommissionBps, &m.OwnerEmail, &m.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "could not read merchants")
			return
		}
		list = append(list, m)
	}
	writeJSON(w, http.StatusOK, map[string]any{"merchants": list})
}

// handleAdminVerifyMerchant approves a merchant so it can start charging.
func (a *App) handleAdminVerifyMerchant(w http.ResponseWriter, r *http.Request) {
	a.adminUpdateMerchant(w, r, `UPDATE merchants SET status = 'verified', reject_reason = '' WHERE id = $1`)
}

// handleAdminRejectMerchant rejects a merchant with a reason.
func (a *App) handleAdminRejectMerchant(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Reason string `json:"reason"`
	}
	_ = decodeJSON(r, &req)
	id := chi.URLParam(r, "id")
	ct, err := a.pool.Exec(r.Context(),
		`UPDATE merchants SET status = 'rejected', reject_reason = $2 WHERE id = $1`,
		id, strings.TrimSpace(req.Reason))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not update merchant")
		return
	}
	if ct.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "comercio no encontrado")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "rejected"})
}

// handleAdminSetCommission adjusts a merchant's commission (basis points).
func (a *App) handleAdminSetCommission(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Bps int `json:"bps"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	if req.Bps < 0 || req.Bps > 10000 {
		writeError(w, http.StatusBadRequest, "la comisión debe estar entre 0 y 10000 puntos básicos")
		return
	}
	id := chi.URLParam(r, "id")
	ct, err := a.pool.Exec(r.Context(),
		`UPDATE merchants SET commission_bps = $2 WHERE id = $1`, id, req.Bps)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not update commission")
		return
	}
	if ct.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "comercio no encontrado")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"commissionBps": req.Bps})
}

func (a *App) adminUpdateMerchant(w http.ResponseWriter, r *http.Request, sql string) {
	id := chi.URLParam(r, "id")
	ct, err := a.pool.Exec(r.Context(), sql, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not update merchant")
		return
	}
	if ct.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "comercio no encontrado")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "verified"})
}
