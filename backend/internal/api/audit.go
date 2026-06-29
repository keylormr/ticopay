package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// auditAction labels a back-office mutation in admin_audit_log. The constants
// are the canonical action vocabulary; the front-end localizes them by key.
type auditAction string

const (
	auditMerchantVerify     auditAction = "merchant.verify"
	auditMerchantReject     auditAction = "merchant.reject"
	auditMerchantCommission auditAction = "merchant.commission"
	auditUserCreate         auditAction = "user.create"
	auditUserSetRole        auditAction = "user.set_role"
	auditUserSetStatus      auditAction = "user.set_status"
)

// marshalDetail renders the per-action context as JSON, never failing the
// caller: an unmarshalable detail (or nil) degrades to an empty object.
func marshalDetail(detail map[string]any) []byte {
	if detail == nil {
		return []byte("{}")
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return []byte("{}")
	}
	return raw
}

// auditTx writes one audit row inside an open transaction, so the trail and the
// mutation it records commit (or roll back) atomically. This is the preferred
// path for governance changes — the log can never drift from what happened.
func auditTx(ctx context.Context, tx pgx.Tx, actorID string, action auditAction, target string, detail map[string]any) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO admin_audit_log (actor_id, action, target, detail) VALUES ($1, $2, $3, $4)`,
		actorID, string(action), target, marshalDetail(detail))
	return err
}

type auditRow struct {
	ID         int64           `json:"id"`
	Action     string          `json:"action"`
	Target     string          `json:"target"`
	Detail     json.RawMessage `json:"detail"`
	CreatedAt  time.Time       `json:"createdAt"`
	ActorEmail string          `json:"actorEmail"`
	ActorName  string          `json:"actorName"`
}

// handleAdminAuditLog returns the back-office audit trail (newest first),
// paginated and optionally filtered by action. Gated on reports.view.
func (a *App) handleAdminAuditLog(w http.ResponseWriter, r *http.Request) {
	limit := qInt(r, "limit", 50, 1, 200)
	offset := qInt(r, "offset", 0, 0, 1_000_000)
	action := strings.TrimSpace(r.URL.Query().Get("action"))

	var total int
	if err := a.pool.QueryRow(r.Context(),
		`SELECT count(*) FROM admin_audit_log WHERE ($1 = '' OR action = $1)`, action,
	).Scan(&total); err != nil {
		writeError(w, http.StatusInternalServerError, "could not load audit log")
		return
	}

	rows, err := a.pool.Query(r.Context(),
		`SELECT l.id, l.action, l.target, l.detail, l.created_at,
		        COALESCE(u.email, ''), COALESCE(u.full_name, '')
		 FROM admin_audit_log l
		 LEFT JOIN users u ON u.id = l.actor_id
		 WHERE ($1 = '' OR l.action = $1)
		 ORDER BY l.id DESC LIMIT $2 OFFSET $3`, action, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load audit log")
		return
	}
	defer rows.Close()

	list := make([]auditRow, 0, limit)
	for rows.Next() {
		var e auditRow
		var detail []byte
		if err := rows.Scan(&e.ID, &e.Action, &e.Target, &detail, &e.CreatedAt, &e.ActorEmail, &e.ActorName); err != nil {
			writeError(w, http.StatusInternalServerError, "could not read audit log")
			return
		}
		e.Detail = detail
		list = append(list, e)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"entries": list, "total": total, "limit": limit, "offset": offset,
	})
}
