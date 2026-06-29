package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"ticopay/backend/internal/models"
)

// handleAdminWhoami answers only for back-office roles (the route is behind
// requirePerm(permBackoffice)) and returns the caller's role plus the
// capability set the client uses to decide what to render. The server still
// enforces every endpoint independently — these capabilities are UX, not a
// security control.
func (a *App) handleAdminWhoami(w http.ResponseWriter, r *http.Request) {
	role := currentRole(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"role":         role,
		"capabilities": roleCapabilities(role),
	})
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

// handleAdminVerifyMerchant approves a merchant so it can start charging. The
// approval and its audit row commit atomically.
func (a *App) handleAdminVerifyMerchant(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := a.inTx(r.Context(), func(tx pgx.Tx) error {
		ct, e := tx.Exec(r.Context(), `UPDATE merchants SET status = 'verified', reject_reason = '' WHERE id = $1`, id)
		if e != nil {
			return errTransferOther
		}
		if ct.RowsAffected() == 0 {
			return errReqNotFound
		}
		return auditTx(r.Context(), tx, userID(r), auditMerchantVerify, id, nil)
	})
	writeMerchantResult(w, err, map[string]any{"status": "verified"})
}

// handleAdminRejectMerchant rejects a merchant with a reason.
func (a *App) handleAdminRejectMerchant(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Reason string `json:"reason"`
	}
	_ = decodeJSON(r, &req)
	id := chi.URLParam(r, "id")
	reason := strings.TrimSpace(req.Reason)
	err := a.inTx(r.Context(), func(tx pgx.Tx) error {
		ct, e := tx.Exec(r.Context(),
			`UPDATE merchants SET status = 'rejected', reject_reason = $2 WHERE id = $1`, id, reason)
		if e != nil {
			return errTransferOther
		}
		if ct.RowsAffected() == 0 {
			return errReqNotFound
		}
		return auditTx(r.Context(), tx, userID(r), auditMerchantReject, id, map[string]any{"reason": reason})
	})
	writeMerchantResult(w, err, map[string]any{"status": "rejected"})
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
	err := a.inTx(r.Context(), func(tx pgx.Tx) error {
		ct, e := tx.Exec(r.Context(), `UPDATE merchants SET commission_bps = $2 WHERE id = $1`, id, req.Bps)
		if e != nil {
			return errTransferOther
		}
		if ct.RowsAffected() == 0 {
			return errReqNotFound
		}
		return auditTx(r.Context(), tx, userID(r), auditMerchantCommission, id, map[string]any{"bps": req.Bps})
	})
	writeMerchantResult(w, err, map[string]any{"commissionBps": req.Bps})
}

// writeMerchantResult maps a merchant-mutation tx outcome to an HTTP response.
func writeMerchantResult(w http.ResponseWriter, err error, ok map[string]any) {
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, ok)
	case errors.Is(err, errReqNotFound):
		writeError(w, http.StatusNotFound, "comercio no encontrado")
	default:
		writeError(w, http.StatusInternalServerError, "could not update merchant")
	}
}
