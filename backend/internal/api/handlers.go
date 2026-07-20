package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
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
	// Fold the caller's back-office capabilities into /me so the client doesn't
	// need a separate /admin/whoami round-trip on every load. The role never
	// leaves the server; only the derived capability flags do, and every
	// endpoint is still enforced server-side (these are UX hints, not access
	// control). A read failure falls back to the least-privileged role.
	role := roleUser
	if err := a.pool.QueryRow(ctx, `SELECT role FROM users WHERE id = $1`, uid).Scan(&role); err != nil {
		role = roleUser
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user": u, "accounts": accounts, "capabilities": roleCapabilities(role),
	})
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

	fp := "send|" + strings.ToLower(to) + "|" + currency + "|" + strconv.FormatInt(amountCents, 10)
	a.idempotent(w, r, idempotencyKey(r), fp, func(tx pgx.Tx) (int, map[string]any, error) {
		txID, newBalance, err := a.transferTx(r.Context(), tx, userID(r), to, currency, amountCents,
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
	// converting twice. postConversionTx posts the balanced FX double-entry on
	// the transaction idempotent() owns, so the key and the money move commit
	// together.
	fp := "convert|" + req.From + "|" + req.To + "|" + strconv.FormatInt(fromCents, 10)
	a.idempotent(w, r, idempotencyKey(r), fp, func(tx pgx.Tx) (int, map[string]any, error) {
		txID, err := a.postConversionTx(r.Context(), tx, uid, req.From, req.To, fromCents, toCentsVal,
			"Conversión "+req.From+" → "+req.To)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusCreated, map[string]any{
			"id": txID, "fromCents": fromCents, "toCents": toCentsVal, "rate": rates.Crc,
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

// transferTx moves money from the sender's account to the recipient identified
// by email/phone, both in the given currency, on the caller's transaction.
// Returns the new transaction id and the sender's resulting balance, recording
// balanced double-entry ledger rows via postPayment. The caller owns the
// transaction so the money move can share a COMMIT with, e.g., an idempotency
// key row.
func (a *App) transferTx(ctx context.Context, tx pgx.Tx, senderID, to, currency string, amountCents int64, description, kind string) (string, int64, error) {
	fromID, fromBalance, err := lockAccount(ctx, tx, senderID, currency)
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
	id, err := a.postPayment(ctx, tx, fromID, toID, currency, amountCents, 0, description, kind)
	if err != nil {
		return "", 0, errTransferOther
	}
	return id, fromBalance - amountCents, nil
}

// transfer wraps transferTx in its own transaction for callers that move money
// standalone (not under idempotent()).
func (a *App) transfer(ctx context.Context, senderID, to, currency string, amountCents int64, description, kind string) (string, int64, error) {
	var txID string
	var newBalance int64
	err := a.inTx(ctx, func(tx pgx.Tx) error {
		id, bal, e := a.transferTx(ctx, tx, senderID, to, currency, amountCents, description, kind)
		txID, newBalance = id, bal
		return e
	})
	if err != nil {
		return "", 0, err
	}
	return txID, newBalance, nil
}

// payOutTx debits the user's wallet for an outgoing payment with no internal
// recipient (e.g. a utility bill) on the caller's transaction. Records a
// transaction with to_account = NULL and a balanced double-entry against
// SYSTEM:CLEARING (the money leaving the platform), so the ledger stays
// reconcilable against balances.
func (a *App) payOutTx(ctx context.Context, tx pgx.Tx, senderID, currency string, amountCents int64, description, kind string) (string, int64, error) {
	fromID, fromBalance, err := lockAccount(ctx, tx, senderID, currency)
	if err != nil {
		return "", 0, errNoSenderAcct
	}
	if fromBalance < amountCents {
		return "", 0, errInsufficient
	}
	id, err := a.postExternalTx(ctx, tx, fromID, currency, amountCents, description, kind)
	if err != nil {
		return "", 0, errTransferOther
	}
	return id, fromBalance - amountCents, nil
}

// payOut wraps payOutTx in its own transaction for standalone callers.
func (a *App) payOut(ctx context.Context, senderID, currency string, amountCents int64, description, kind string) (string, int64, error) {
	var txID string
	var newBalance int64
	err := a.inTx(ctx, func(tx pgx.Tx) error {
		id, bal, e := a.payOutTx(ctx, tx, senderID, currency, amountCents, description, kind)
		txID, newBalance = id, bal
		return e
	})
	if err != nil {
		return "", 0, err
	}
	return txID, newBalance, nil
}
