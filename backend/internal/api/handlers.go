package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"ticopay/backend/internal/models"
)

func (a *App) fetchUser(ctx context.Context, uid string) (models.User, error) {
	var u models.User
	err := a.pool.QueryRow(ctx,
		`SELECT id, email, COALESCE(phone,''), full_name, kyc_status,
		        COALESCE(id_type,''), COALESCE(id_number,''), COALESCE(email_verified,false), created_at
		 FROM users WHERE id = $1`, uid,
	).Scan(&u.ID, &u.Email, &u.Phone, &u.FullName, &u.KYCStatus, &u.IDType, &u.IDNumber, &u.EmailVerified, &u.CreatedAt)
	return u, err
}

// tokenVersion returns the user's current JWT generation. Tokens are stamped
// with it at issue time and rejected once it changes (see requireAuth).
func (a *App) tokenVersion(ctx context.Context, uid string) (int, error) {
	var ver int
	err := a.pool.QueryRow(ctx, `SELECT token_version FROM users WHERE id = $1`, uid).Scan(&ver)
	return ver, err
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := userID(r)

	u, err := a.fetchUser(ctx, uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	accounts, err := a.fetchAccounts(ctx, uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load accounts")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": u, "accounts": accounts})
}

func (a *App) handleListTransactions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := userID(r)

	rows, err := a.pool.Query(ctx, `
		SELECT t.id,
		       CASE WHEN fa.user_id = $1 AND ta.user_id = $1 THEN 'self'
		            WHEN fa.user_id = $1 THEN 'out'
		            ELSE 'in' END AS direction,
		       COALESCE(CASE WHEN fa.user_id = $1 THEN tu.full_name ELSE fu.full_name END, 'Tico Pay') AS counterpart,
		       t.amount_cents, t.currency, t.description, t.status, t.kind, t.created_at
		FROM transactions t
		LEFT JOIN accounts fa ON fa.id = t.from_account_id
		LEFT JOIN accounts ta ON ta.id = t.to_account_id
		LEFT JOIN users fu ON fu.id = fa.user_id
		LEFT JOIN users tu ON tu.id = ta.user_id
		WHERE fa.user_id = $1 OR ta.user_id = $1
		ORDER BY t.created_at DESC
		LIMIT 100`, uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load transactions")
		return
	}
	defer rows.Close()

	txs := make([]models.Transaction, 0)
	for rows.Next() {
		var t models.Transaction
		if err := rows.Scan(&t.ID, &t.Direction, &t.Counterpart, &t.AmountCents,
			&t.Currency, &t.Description, &t.Status, &t.Kind, &t.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "could not read transactions")
			return
		}
		txs = append(txs, t)
	}
	writeJSON(w, http.StatusOK, map[string]any{"transactions": txs})
}

func (a *App) handleSendMoney(w http.ResponseWriter, r *http.Request) {
	var req struct {
		To          string  `json:"to"`       // email OR phone
		ToEmail     string  `json:"toEmail"`  // legacy alias
		Amount      float64 `json:"amount"`   // major units (colones / dollars)
		Currency    string  `json:"currency"` // CRC | USD
		Description string  `json:"description"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	to := req.To
	if to == "" {
		to = req.ToEmail
	}
	to = strings.TrimSpace(to)
	currency := req.Currency
	if currency == "" {
		currency = "CRC"
	}
	if !validCurrency(currency) {
		writeError(w, http.StatusBadRequest, "moneda no soportada")
		return
	}
	amountCents := toMinor(req.Amount, currency)
	if amountCents <= 0 {
		writeError(w, http.StatusBadRequest, "el monto debe ser mayor a cero")
		return
	}
	if to == "" {
		writeError(w, http.StatusBadRequest, "el destinatario (correo o teléfono) es obligatorio")
		return
	}

	a.idempotent(w, r, idempotencyKey(r), func() (int, map[string]any, error) {
		txID, newBalance, err := a.transfer(r.Context(), userID(r), to, currency, amountCents,
			strings.TrimSpace(req.Description), "transfer")
		if err != nil {
			return 0, nil, err
		}
		return http.StatusCreated, map[string]any{
			"id": txID, "amountCents": amountCents, "currency": currency, "newBalance": newBalance,
		}, nil
	})
}

func (a *App) handleConvert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From   string  `json:"from"`
		To     string  `json:"to"`
		Amount float64 `json:"amount"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	if !validCurrency(req.From) || !validCurrency(req.To) || req.From == req.To {
		writeError(w, http.StatusBadRequest, "elegí dos monedas distintas")
		return
	}
	fromCents := toMinor(req.Amount, req.From)
	if fromCents <= 0 {
		writeError(w, http.StatusBadRequest, "el monto debe ser mayor a cero")
		return
	}

	// Convert through a common USD reference so any pair (fiat or crypto) works.
	rates := a.getRates(r.Context())
	upFrom, upTo := rates.UsdPerUnit[req.From], rates.UsdPerUnit[req.To]
	if upFrom <= 0 || upTo <= 0 {
		writeError(w, http.StatusServiceUnavailable, "tipo de cambio no disponible")
		return
	}
	usdValue := majorOf(fromCents, req.From) * upFrom
	toCentsVal := toMinor(usdValue/upTo, req.To)
	if toCentsVal <= 0 {
		writeError(w, http.StatusBadRequest, "monto muy pequeño para convertir")
		return
	}

	uid := userID(r)
	// Idempotent so a retried/double-submitted conversion replays instead of
	// converting twice. The money move (both legs + the transactions row) runs
	// in a single transaction with both accounts locked.
	a.idempotent(w, r, idempotencyKey(r), func() (int, map[string]any, error) {
		ctx := r.Context()
		tx, err := a.pool.Begin(ctx)
		if err != nil {
			return 0, nil, errTransferOther
		}
		defer tx.Rollback(ctx)

		// Lock both of the user's accounts.
		rows, err := tx.Query(ctx,
			`SELECT id, currency, balance_cents FROM accounts
			 WHERE user_id = $1 AND currency IN ($2, $3) ORDER BY currency FOR UPDATE`,
			uid, req.From, req.To)
		if err != nil {
			return 0, nil, errTransferOther
		}
		accByCur := map[string]struct {
			id  string
			bal int64
		}{}
		for rows.Next() {
			var id, cur string
			var bal int64
			if err := rows.Scan(&id, &cur, &bal); err != nil {
				rows.Close()
				return 0, nil, errTransferOther
			}
			accByCur[cur] = struct {
				id  string
				bal int64
			}{id, bal}
		}
		rows.Close()

		from, okF := accByCur[req.From]
		dst, okT := accByCur[req.To]
		if !okF || !okT {
			return 0, nil, errTransferOther
		}
		if from.bal < fromCents {
			return 0, nil, errInsufficient
		}

		if _, err := tx.Exec(ctx, `UPDATE accounts SET balance_cents = balance_cents - $1 WHERE id = $2`, fromCents, from.id); err != nil {
			return 0, nil, errTransferOther
		}
		if _, err := tx.Exec(ctx, `UPDATE accounts SET balance_cents = balance_cents + $1 WHERE id = $2`, toCentsVal, dst.id); err != nil {
			return 0, nil, errTransferOther
		}
		desc := "Conversión " + req.From + " → " + req.To
		if _, err := tx.Exec(ctx,
			`INSERT INTO transactions (from_account_id, to_account_id, amount_cents, currency, description, status, kind)
			 VALUES ($1, $2, $3, $4, $5, 'completed', 'conversion')`,
			from.id, dst.id, fromCents, req.From, desc); err != nil {
			return 0, nil, errTransferOther
		}
		if err := tx.Commit(ctx); err != nil {
			return 0, nil, errTransferOther
		}
		return http.StatusCreated, map[string]any{
			"fromCents": fromCents, "toCents": toCentsVal, "rate": rates.Crc,
		}, nil
	})
}

// --- shared transfer logic (used by send, request-pay, pool-contribute) ---

var (
	errSelfTransfer  = errors.New("self transfer")
	errNoRecipient   = errors.New("recipient not found")
	errInsufficient  = errors.New("insufficient balance")
	errNoSenderAcct  = errors.New("sender account missing")
	errTransferOther = errors.New("transfer failed")
)

// writeTransferError maps every money-path sentinel to an HTTP status. Used by
// send, SINPE, service payment, cobros, vaquitas and merchant charges.
func writeTransferError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errNoRecipient):
		writeError(w, http.StatusNotFound, "destinatario no encontrado")
	case errors.Is(err, errReqNotFound):
		writeError(w, http.StatusNotFound, "cobro no encontrado")
	case errors.Is(err, errPoolNotFound):
		writeError(w, http.StatusNotFound, "vaquita no encontrada")
	case errors.Is(err, errSelfTransfer):
		writeError(w, http.StatusBadRequest, "no podés enviarte dinero a vos mismo")
	case errors.Is(err, errInsufficient):
		writeError(w, http.StatusBadRequest, "saldo insuficiente")
	case errors.Is(err, errBadAmount):
		writeError(w, http.StatusBadRequest, "el monto debe ser mayor a cero")
	case errors.Is(err, errReqNotPayable):
		writeError(w, http.StatusConflict, "este cobro ya fue pagado o cancelado")
	case errors.Is(err, errPoolClosed):
		writeError(w, http.StatusConflict, "esta vaquita está cerrada")
	case errors.Is(err, errMerchantUnverified):
		writeError(w, http.StatusForbidden, "el comercio aún no está verificado")
	default:
		writeError(w, http.StatusInternalServerError, "no se pudo completar la operación")
	}
}

// transfer moves money from the sender's account to the recipient identified by
// email/phone, both in the given currency. Returns the new transaction id and
// the sender's resulting balance. Runs as a single transaction and records
// balanced double-entry ledger rows via postPayment.
func (a *App) transfer(ctx context.Context, senderID, to, currency string, amountCents int64, description, kind string) (string, int64, error) {
	var txID string
	var newBalance int64
	err := a.inTx(ctx, func(tx pgx.Tx) error {
		fromID, fromBalance, err := lockAccount(ctx, tx, senderID, currency)
		if err != nil {
			return errNoSenderAcct
		}
		toID, toUserID, _, err := resolveRecipientAccount(ctx, tx, to, currency)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNoRecipient
		}
		if err != nil {
			return errTransferOther
		}
		if toUserID == senderID {
			return errSelfTransfer
		}
		if fromBalance < amountCents {
			return errInsufficient
		}
		id, err := a.postPayment(ctx, tx, fromID, toID, currency, amountCents, 0, description, kind)
		if err != nil {
			return errTransferOther
		}
		txID = id
		newBalance = fromBalance - amountCents
		return nil
	})
	if err != nil {
		return "", 0, err
	}
	return txID, newBalance, nil
}

// transferToUser moves money from sender to a known recipient user id (used by
// paid requests and pool contributions). Returns the transaction id. The
// per-tx variant transferToUserTx (ledger.go) lets callers move money and
// update business rows atomically.
func (a *App) transferToUser(ctx context.Context, senderID, recipientID, currency string, amountCents int64, description, kind string) (string, error) {
	var txID string
	err := a.inTx(ctx, func(tx pgx.Tx) error {
		id, err := a.transferToUserTx(ctx, tx, senderID, recipientID, currency, amountCents, 0, description, kind)
		txID = id
		return err
	})
	return txID, err
}

// payOut debits the user's wallet for an outgoing payment with no internal
// recipient (e.g. a utility bill). Records a transaction with to_account = NULL.
// External settlements are not ledgered (no internal counterpart); balances
// remain the source of truth.
func (a *App) payOut(ctx context.Context, senderID, currency string, amountCents int64, description, kind string) (string, int64, error) {
	var txID string
	var newBalance int64
	err := a.inTx(ctx, func(tx pgx.Tx) error {
		fromID, fromBalance, err := lockAccount(ctx, tx, senderID, currency)
		if err != nil {
			return errNoSenderAcct
		}
		if fromBalance < amountCents {
			return errInsufficient
		}
		if _, err := tx.Exec(ctx, `UPDATE accounts SET balance_cents = balance_cents - $1 WHERE id = $2`, amountCents, fromID); err != nil {
			return errTransferOther
		}
		if err := tx.QueryRow(ctx,
			`INSERT INTO transactions (from_account_id, to_account_id, amount_cents, currency, description, status, kind)
			 VALUES ($1, NULL, $2, $3, $4, 'completed', $5) RETURNING id`,
			fromID, amountCents, currency, description, kind,
		).Scan(&txID); err != nil {
			return errTransferOther
		}
		newBalance = fromBalance - amountCents
		return nil
	})
	if err != nil {
		return "", 0, err
	}
	return txID, newBalance, nil
}
