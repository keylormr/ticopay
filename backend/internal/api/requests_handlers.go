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
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	currency := req.Currency
	if currency == "" {
		currency = "CRC"
	}
	if !validCurrency(currency) {
		writeError(w, http.StatusBadRequest, "moneda no soportada")
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
	payer := userID(r)
	var (
		respAmount   int64
		respCurrency string
		feeApplied   int64
	)
	// The whole settlement runs in one transaction: the cobro row is locked
	// FOR UPDATE, its status re-checked under the lock, the money moved, and the
	// status flipped to paid — so concurrent or retried calls can't double-pay.
	err := a.inTx(ctx, func(tx pgx.Tx) error {
		var (
			requesterID string
			amountCents *int64
			currency    string
			status      string
			description string
			merchantID  *string
			paidBy      *string
			paidTxID    *string
		)
		err := tx.QueryRow(ctx,
			`SELECT requester_id, amount_cents, currency, status, description, merchant_id, paid_by, paid_tx_id
			 FROM payment_requests WHERE id = $1 FOR UPDATE`, id,
		).Scan(&requesterID, &amountCents, &currency, &status, &description, &merchantID, &paidBy, &paidTxID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errReqNotFound
		}
		if err != nil {
			return errTransferOther
		}

		// Idempotent replay: if THIS payer already settled the cobro, report the
		// same result instead of charging again (a retried request is a no-op).
		// Read the real amounts from the original transaction so the replayed
		// receipt matches — including merchant commission and open amounts.
		if status == "paid" {
			if paidBy != nil && *paidBy == payer {
				respCurrency = currency
				if paidTxID != nil {
					_ = tx.QueryRow(ctx,
						`SELECT amount_cents, fee_cents FROM transactions WHERE id = $1`, *paidTxID,
					).Scan(&respAmount, &feeApplied)
				} else if amountCents != nil {
					respAmount = *amountCents
				}
				return nil
			}
			return errReqNotPayable
		}
		if status != "pending" {
			return errReqNotPayable
		}

		pay := int64(0)
		if amountCents != nil {
			pay = *amountCents
		} else {
			pay = toMinor(body.Amount, currency)
		}
		if pay <= 0 {
			return errBadAmount
		}

		desc := description
		if desc == "" {
			desc = "Pago de cobro"
		}
		// Merchant cobros carry a commission absorbed by the merchant; the payer
		// pays exactly `pay` and the merchant receives pay - fee.
		fee := int64(0)
		kind := "request"
		if merchantID != nil {
			var mstatus string
			var bps int64
			if err := tx.QueryRow(ctx,
				`SELECT status, commission_bps FROM merchants WHERE id = $1 FOR UPDATE`, *merchantID,
			).Scan(&mstatus, &bps); err != nil {
				return errTransferOther
			}
			if mstatus != "verified" {
				return errMerchantUnverified
			}
			fee = feeCents(pay, bps)
			kind = "merchant"
		}

		txID, err := a.transferToUserTx(ctx, tx, payer, requesterID, currency, pay, fee, desc, kind)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE payment_requests SET status = 'paid', paid_by = $1, paid_tx_id = $2 WHERE id = $3`,
			payer, txID, id,
		); err != nil {
			return errTransferOther
		}
		respAmount = pay
		respCurrency = currency
		feeApplied = fee
		return nil
	})
	if err != nil {
		writeTransferError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "paid", "amountCents": respAmount, "currency": respCurrency,
		"feeCents": feeApplied, "netCents": respAmount - feeApplied,
	})
}
