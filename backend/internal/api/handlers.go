package api

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"ticopay/backend/internal/models"
)

func (a *App) fetchAccount(ctx context.Context, uid string) (models.Account, error) {
	var acc models.Account
	err := a.pool.QueryRow(ctx,
		`SELECT id, currency, balance_cents FROM accounts WHERE user_id = $1 AND currency = 'CRC'`,
		uid,
	).Scan(&acc.ID, &acc.Currency, &acc.BalanceCents)
	return acc, err
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := userID(r)

	var u models.User
	err := a.pool.QueryRow(ctx,
		`SELECT id, email, COALESCE(phone,''), full_name, created_at FROM users WHERE id = $1`, uid,
	).Scan(&u.ID, &u.Email, &u.Phone, &u.FullName, &u.CreatedAt)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	acc, err := a.fetchAccount(ctx, uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load account")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"user": u, "account": acc})
}

func (a *App) handleListTransactions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	acc, err := a.fetchAccount(ctx, userID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load account")
		return
	}

	rows, err := a.pool.Query(ctx, `
		SELECT t.id,
		       CASE WHEN t.from_account_id = $1 THEN 'out' ELSE 'in' END AS direction,
		       COALESCE(cp.full_name, 'Tico Pay') AS counterpart,
		       t.amount_cents, t.currency, t.description, t.status, t.created_at
		FROM transactions t
		LEFT JOIN accounts ca
		       ON ca.id = CASE WHEN t.from_account_id = $1 THEN t.to_account_id ELSE t.from_account_id END
		LEFT JOIN users cp ON cp.id = ca.user_id
		WHERE t.from_account_id = $1 OR t.to_account_id = $1
		ORDER BY t.created_at DESC
		LIMIT 100`, acc.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load transactions")
		return
	}
	defer rows.Close()

	txs := make([]models.Transaction, 0)
	for rows.Next() {
		var t models.Transaction
		if err := rows.Scan(&t.ID, &t.Direction, &t.Counterpart, &t.AmountCents,
			&t.Currency, &t.Description, &t.Status, &t.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "could not read transactions")
			return
		}
		txs = append(txs, t)
	}
	writeJSON(w, http.StatusOK, map[string]any{"transactions": txs})
}

func (a *App) handleSendMoney(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ToEmail     string  `json:"toEmail"`
		Amount      float64 `json:"amount"` // in colones
		Description string  `json:"description"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.ToEmail = strings.ToLower(strings.TrimSpace(req.ToEmail))
	amountCents := int64(math.Round(req.Amount * 100))
	if amountCents <= 0 {
		writeError(w, http.StatusBadRequest, "amount must be greater than zero")
		return
	}
	if req.ToEmail == "" {
		writeError(w, http.StatusBadRequest, "recipient email is required")
		return
	}

	ctx := r.Context()
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer tx.Rollback(ctx)

	// Sender account (lock the row to serialize concurrent transfers).
	var fromID string
	var fromBalance int64
	if err := tx.QueryRow(ctx,
		`SELECT id, balance_cents FROM accounts WHERE user_id = $1 AND currency = 'CRC' FOR UPDATE`,
		userID(r),
	).Scan(&fromID, &fromBalance); err != nil {
		writeError(w, http.StatusInternalServerError, "could not load your account")
		return
	}

	// Recipient account by email.
	var toID, toUserID string
	err = tx.QueryRow(ctx,
		`SELECT acc.id, u.id
		 FROM users u JOIN accounts acc ON acc.user_id = u.id AND acc.currency = 'CRC'
		 WHERE u.email = $1`, req.ToEmail,
	).Scan(&toID, &toUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "recipient not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load recipient")
		return
	}
	if toUserID == userID(r) {
		writeError(w, http.StatusBadRequest, "you cannot send money to yourself")
		return
	}
	if fromBalance < amountCents {
		writeError(w, http.StatusBadRequest, "insufficient balance")
		return
	}

	if _, err := tx.Exec(ctx,
		`UPDATE accounts SET balance_cents = balance_cents - $1 WHERE id = $2`, amountCents, fromID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not debit account")
		return
	}
	if _, err := tx.Exec(ctx,
		`UPDATE accounts SET balance_cents = balance_cents + $1 WHERE id = $2`, amountCents, toID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not credit recipient")
		return
	}

	var txID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO transactions (from_account_id, to_account_id, amount_cents, currency, description, status)
		 VALUES ($1, $2, $3, 'CRC', $4, 'completed') RETURNING id`,
		fromID, toID, amountCents, strings.TrimSpace(req.Description),
	).Scan(&txID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not record transaction")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "could not complete transfer")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":          txID,
		"amountCents": amountCents,
		"newBalance":  fromBalance - amountCents,
	})
}
