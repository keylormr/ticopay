package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"ticopay/backend/internal/models"
)

// resolveUserID looks up a user id + name by email or phone.
func (a *App) resolveUserID(ctx context.Context, to string) (id, name string, err error) {
	email, phone := parseRecipient(to)
	err = a.pool.QueryRow(ctx, `
		SELECT id, full_name FROM users
		WHERE ($1 <> '' AND lower(email) = $1)
		   OR ($2 <> '' AND regexp_replace(COALESCE(phone,''), '\D', '', 'g') = $2)
		LIMIT 1`, email, phone).Scan(&id, &name)
	return id, name, err
}

func (a *App) handleCreateRequest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		To          string  `json:"to"` // optional: target payer by email/phone
		Amount      float64 `json:"amount"`
		Currency    string  `json:"currency"`
		Description string  `json:"description"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	currency := req.Currency
	if currency == "" {
		currency = "CRC"
	}
	if !validCurrency(currency) {
		writeError(w, http.StatusBadRequest, "unsupported currency")
		return
	}

	var amountCents *int64
	if req.Amount > 0 {
		c := toMinor(req.Amount, currency)
		amountCents = &c
	}

	ctx := r.Context()
	var targetID *string
	if to := strings.TrimSpace(req.To); to != "" {
		id, _, err := a.resolveUserID(ctx, to)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "no encontramos a esa persona en Tico Pay")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not resolve target")
			return
		}
		targetID = &id
	}

	var id string
	if err := a.pool.QueryRow(ctx,
		`INSERT INTO payment_requests (requester_id, target_user_id, amount_cents, currency, description)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		userID(r), targetID, amountCents, currency, strings.TrimSpace(req.Description),
	).Scan(&id); err != nil {
		writeError(w, http.StatusInternalServerError, "could not create request")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "currency": currency})
}

func (a *App) handleListRequests(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := userID(r)

	// Outgoing: requests I created (counterpart = target or "Cualquiera").
	outgoing, err := a.queryRequests(ctx, `
		SELECT pr.id, COALESCE(tu.full_name, 'Cualquiera'), pr.amount_cents, pr.currency,
		       pr.description, pr.status, pr.created_at
		FROM payment_requests pr
		LEFT JOIN users tu ON tu.id = pr.target_user_id
		WHERE pr.requester_id = $1
		ORDER BY pr.created_at DESC LIMIT 50`, uid, "outgoing")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load requests")
		return
	}

	// Incoming: requests targeted at me that are still pending.
	incoming, err := a.queryRequests(ctx, `
		SELECT pr.id, ru.full_name, pr.amount_cents, pr.currency,
		       pr.description, pr.status, pr.created_at
		FROM payment_requests pr
		JOIN users ru ON ru.id = pr.requester_id
		WHERE pr.target_user_id = $1 AND pr.status = 'pending'
		ORDER BY pr.created_at DESC LIMIT 50`, uid, "incoming")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load requests")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"incoming": incoming, "outgoing": outgoing})
}

func (a *App) queryRequests(ctx context.Context, sql, uid, direction string) ([]models.PaymentRequest, error) {
	rows, err := a.pool.Query(ctx, sql, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]models.PaymentRequest, 0)
	for rows.Next() {
		var pr models.PaymentRequest
		if err := rows.Scan(&pr.ID, &pr.RequesterName, &pr.AmountCents, &pr.Currency,
			&pr.Description, &pr.Status, &pr.CreatedAt); err != nil {
			return nil, err
		}
		pr.Direction = direction
		list = append(list, pr)
	}
	return list, rows.Err()
}

func (a *App) handleGetRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var pr models.PaymentRequest
	err := a.pool.QueryRow(r.Context(), `
		SELECT pr.id, ru.full_name, pr.amount_cents, pr.currency, pr.description, pr.status, pr.created_at
		FROM payment_requests pr JOIN users ru ON ru.id = pr.requester_id
		WHERE pr.id = $1`, id,
	).Scan(&pr.ID, &pr.RequesterName, &pr.AmountCents, &pr.Currency, &pr.Description, &pr.Status, &pr.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "cobro no encontrado")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load request")
		return
	}
	writeJSON(w, http.StatusOK, pr)
}

func (a *App) handlePayRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Amount float64 `json:"amount"` // used only if the request has no fixed amount
	}
	_ = decodeJSON(r, &body)

	ctx := r.Context()
	var (
		requesterID string
		amountCents *int64
		currency    string
		status      string
		description string
	)
	err := a.pool.QueryRow(ctx,
		`SELECT requester_id, amount_cents, currency, status, description FROM payment_requests WHERE id = $1`, id,
	).Scan(&requesterID, &amountCents, &currency, &status, &description)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "cobro no encontrado")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load request")
		return
	}
	if status != "pending" {
		writeError(w, http.StatusConflict, "este cobro ya fue pagado o cancelado")
		return
	}

	pay := int64(0)
	if amountCents != nil {
		pay = *amountCents
	} else {
		pay = toMinor(body.Amount, currency)
	}
	if pay <= 0 {
		writeError(w, http.StatusBadRequest, "amount must be greater than zero")
		return
	}

	desc := description
	if desc == "" {
		desc = "Pago de cobro"
	}
	txID, err := a.transferToUser(ctx, userID(r), requesterID, currency, pay, desc, "request")
	if err != nil {
		writeTransferError(w, err)
		return
	}

	if _, err := a.pool.Exec(ctx,
		`UPDATE payment_requests SET status = 'paid', paid_by = $1, paid_tx_id = $2 WHERE id = $3`,
		userID(r), txID, id,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "payment recorded but request not updated")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "paid", "amountCents": pay, "currency": currency})
}
