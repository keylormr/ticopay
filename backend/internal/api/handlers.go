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

	txID, newBalance, err := a.transfer(r.Context(), userID(r), to, currency, amountCents,
		strings.TrimSpace(req.Description), "transfer")
	if err != nil {
		writeTransferError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": txID, "amountCents": amountCents, "currency": currency, "newBalance": newBalance,
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

	ctx := r.Context()
	uid := userID(r)
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer tx.Rollback(ctx)

	// Lock both of the user's accounts.
	rows, err := tx.Query(ctx,
		`SELECT id, currency, balance_cents FROM accounts
		 WHERE user_id = $1 AND currency IN ($2, $3) ORDER BY currency FOR UPDATE`,
		uid, req.From, req.To)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load accounts")
		return
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
			writeError(w, http.StatusInternalServerError, "could not read accounts")
			return
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
		writeError(w, http.StatusInternalServerError, "missing account for currency")
		return
	}
	if from.bal < fromCents {
		writeError(w, http.StatusBadRequest, "saldo insuficiente")
		return
	}

	if _, err := tx.Exec(ctx, `UPDATE accounts SET balance_cents = balance_cents - $1 WHERE id = $2`, fromCents, from.id); err != nil {
		writeError(w, http.StatusInternalServerError, "could not debit account")
		return
	}
	if _, err := tx.Exec(ctx, `UPDATE accounts SET balance_cents = balance_cents + $1 WHERE id = $2`, toCentsVal, dst.id); err != nil {
		writeError(w, http.StatusInternalServerError, "could not credit account")
		return
	}
	desc := "Conversión " + req.From + " → " + req.To
	if _, err := tx.Exec(ctx,
		`INSERT INTO transactions (from_account_id, to_account_id, amount_cents, currency, description, status, kind)
		 VALUES ($1, $2, $3, $4, $5, 'completed', 'conversion')`,
		from.id, dst.id, fromCents, req.From, desc); err != nil {
		writeError(w, http.StatusInternalServerError, "could not record conversion")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "could not complete conversion")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"fromCents": fromCents, "toCents": toCentsVal, "rate": rates.Crc,
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

func writeTransferError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errNoRecipient):
		writeError(w, http.StatusNotFound, "destinatario no encontrado")
	case errors.Is(err, errSelfTransfer):
		writeError(w, http.StatusBadRequest, "no podés enviarte dinero a vos mismo")
	case errors.Is(err, errInsufficient):
		writeError(w, http.StatusBadRequest, "saldo insuficiente")
	default:
		writeError(w, http.StatusInternalServerError, "no se pudo completar la operación")
	}
}

// transfer moves money from the sender's account to the recipient identified by
// email/phone, both in the given currency. Returns the new transaction id and
// the sender's resulting balance.
func (a *App) transfer(ctx context.Context, senderID, to, currency string, amountCents int64, description, kind string) (string, int64, error) {
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return "", 0, errTransferOther
	}
	defer tx.Rollback(ctx)

	var fromID string
	var fromBalance int64
	err = tx.QueryRow(ctx,
		`SELECT id, balance_cents FROM accounts WHERE user_id = $1 AND currency = $2 FOR UPDATE`,
		senderID, currency,
	).Scan(&fromID, &fromBalance)
	if err != nil {
		return "", 0, errNoSenderAcct
	}

	toID, toUserID, _, err := resolveRecipientAccount(ctx, tx, to, currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, errNoRecipient
	}
	if err != nil {
		return "", 0, errTransferOther
	}
	if toUserID == senderID {
		return "", 0, errSelfTransfer
	}
	if fromBalance < amountCents {
		return "", 0, errInsufficient
	}

	if _, err := tx.Exec(ctx, `UPDATE accounts SET balance_cents = balance_cents - $1 WHERE id = $2`, amountCents, fromID); err != nil {
		return "", 0, errTransferOther
	}
	if _, err := tx.Exec(ctx, `UPDATE accounts SET balance_cents = balance_cents + $1 WHERE id = $2`, amountCents, toID); err != nil {
		return "", 0, errTransferOther
	}
	var txID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO transactions (from_account_id, to_account_id, amount_cents, currency, description, status, kind)
		 VALUES ($1, $2, $3, $4, $5, 'completed', $6) RETURNING id`,
		fromID, toID, amountCents, currency, description, kind,
	).Scan(&txID); err != nil {
		return "", 0, errTransferOther
	}
	if err := tx.Commit(ctx); err != nil {
		return "", 0, errTransferOther
	}
	return txID, fromBalance - amountCents, nil
}

// transferToUser moves money from sender to a known recipient user id (used by
// paid requests and pool contributions). Returns the transaction id.
func (a *App) transferToUser(ctx context.Context, senderID, recipientID, currency string, amountCents int64, description, kind string) (string, error) {
	if recipientID == senderID {
		return "", errSelfTransfer
	}
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return "", errTransferOther
	}
	defer tx.Rollback(ctx)

	var fromID string
	var fromBalance int64
	if err := tx.QueryRow(ctx,
		`SELECT id, balance_cents FROM accounts WHERE user_id = $1 AND currency = $2 FOR UPDATE`,
		senderID, currency,
	).Scan(&fromID, &fromBalance); err != nil {
		return "", errNoSenderAcct
	}

	var toID string
	if err := tx.QueryRow(ctx,
		`SELECT id FROM accounts WHERE user_id = $1 AND currency = $2 FOR UPDATE`,
		recipientID, currency,
	).Scan(&toID); err != nil {
		return "", errNoRecipient
	}
	if fromBalance < amountCents {
		return "", errInsufficient
	}

	if _, err := tx.Exec(ctx, `UPDATE accounts SET balance_cents = balance_cents - $1 WHERE id = $2`, amountCents, fromID); err != nil {
		return "", errTransferOther
	}
	if _, err := tx.Exec(ctx, `UPDATE accounts SET balance_cents = balance_cents + $1 WHERE id = $2`, amountCents, toID); err != nil {
		return "", errTransferOther
	}
	var txID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO transactions (from_account_id, to_account_id, amount_cents, currency, description, status, kind)
		 VALUES ($1, $2, $3, $4, $5, 'completed', $6) RETURNING id`,
		fromID, toID, amountCents, currency, description, kind,
	).Scan(&txID); err != nil {
		return "", errTransferOther
	}
	if err := tx.Commit(ctx); err != nil {
		return "", errTransferOther
	}
	return txID, nil
}

// payOut debits the user's wallet for an outgoing payment with no internal
// recipient (e.g. a utility bill). Records a transaction with to_account = NULL.
func (a *App) payOut(ctx context.Context, senderID, currency string, amountCents int64, description, kind string) (string, int64, error) {
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return "", 0, errTransferOther
	}
	defer tx.Rollback(ctx)

	var fromID string
	var fromBalance int64
	if err := tx.QueryRow(ctx,
		`SELECT id, balance_cents FROM accounts WHERE user_id = $1 AND currency = $2 FOR UPDATE`,
		senderID, currency,
	).Scan(&fromID, &fromBalance); err != nil {
		return "", 0, errNoSenderAcct
	}
	if fromBalance < amountCents {
		return "", 0, errInsufficient
	}
	if _, err := tx.Exec(ctx, `UPDATE accounts SET balance_cents = balance_cents - $1 WHERE id = $2`, amountCents, fromID); err != nil {
		return "", 0, errTransferOther
	}
	var txID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO transactions (from_account_id, to_account_id, amount_cents, currency, description, status, kind)
		 VALUES ($1, NULL, $2, $3, $4, 'completed', $5) RETURNING id`,
		fromID, amountCents, currency, description, kind,
	).Scan(&txID); err != nil {
		return "", 0, errTransferOther
	}
	if err := tx.Commit(ctx); err != nil {
		return "", 0, errTransferOther
	}
	return txID, fromBalance - amountCents, nil
}
