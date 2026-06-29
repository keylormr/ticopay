package api

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// sysFeesUserID is the reserved system user (see migration 0012) that owns the
// platform fee accounts. Commission credits land here.
const sysFeesUserID = "00000000-0000-0000-0000-0000000000fe"

var (
	errReqNotFound        = errors.New("request not found")
	errReqNotPayable      = errors.New("request not payable")
	errPoolNotFound       = errors.New("pool not found")
	errPoolClosed         = errors.New("pool closed")
	errMerchantUnverified = errors.New("merchant not verified")
	errBadAmount          = errors.New("bad amount")
)

// inTx runs fn inside a single database transaction, rolling back on any error
// and committing otherwise. Errors from fn propagate verbatim so handlers can
// map domain sentinels; only Begin/Commit failures map to errTransferOther.
func (a *App) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return errTransferOther
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return errTransferOther
	}
	return nil
}

// lockAccount locks a user's account for a currency (FOR UPDATE) and returns its
// id and current balance, serializing concurrent debits on the same account.
func lockAccount(ctx context.Context, tx pgx.Tx, userID, currency string) (id string, balance int64, err error) {
	err = tx.QueryRow(ctx,
		`SELECT id, balance_cents FROM accounts WHERE user_id = $1 AND currency = $2 FOR UPDATE`,
		userID, currency,
	).Scan(&id, &balance)
	return id, balance, err
}

// ensureSystemAccount returns the system user's account id for a currency,
// creating it on first use. The DO UPDATE no-op lets RETURNING fire on conflict.
func ensureSystemAccount(ctx context.Context, tx pgx.Tx, sysUserID, currency string) (string, error) {
	var id string
	err := tx.QueryRow(ctx,
		`INSERT INTO accounts (user_id, currency, balance_cents) VALUES ($1, $2, 0)
		 ON CONFLICT (user_id, currency) DO UPDATE SET currency = EXCLUDED.currency
		 RETURNING id`,
		sysUserID, currency,
	).Scan(&id)
	return id, err
}

// feeCents returns the integer commission in minor units: amount * bps / 10000,
// truncated to the cent. Integer arithmetic only — no float drift.
func feeCents(amount, bps int64) int64 {
	if bps <= 0 || amount <= 0 {
		return 0
	}
	return amount * bps / 10000
}

// postPayment debits fromAccID by amount, credits toAccID by (amount - fee) and
// the SYSTEM:FEES account by fee, writing one transactions row plus balanced
// double-entry ledger rows (validated at COMMIT by the ledger_balanced trigger).
// Callers must have locked fromAccID (FOR UPDATE) and verified its balance.
// fee must be in [0, amount].
func (a *App) postPayment(ctx context.Context, tx pgx.Tx, fromAccID, toAccID, currency string, amount, fee int64, description, kind string) (string, error) {
	if amount <= 0 || fee < 0 || fee > amount {
		return "", errTransferOther
	}
	net := amount - fee

	if _, err := tx.Exec(ctx, `UPDATE accounts SET balance_cents = balance_cents - $1 WHERE id = $2`, amount, fromAccID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `UPDATE accounts SET balance_cents = balance_cents + $1 WHERE id = $2`, net, toAccID); err != nil {
		return "", err
	}

	var feeAccID string
	if fee > 0 {
		var err error
		if feeAccID, err = ensureSystemAccount(ctx, tx, sysFeesUserID, currency); err != nil {
			return "", err
		}
		if _, err := tx.Exec(ctx, `UPDATE accounts SET balance_cents = balance_cents + $1 WHERE id = $2`, fee, feeAccID); err != nil {
			return "", err
		}
	}

	var txID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO transactions (from_account_id, to_account_id, amount_cents, currency, description, status, kind, fee_cents)
		 VALUES ($1, $2, $3, $4, $5, 'completed', $6, $7) RETURNING id`,
		fromAccID, toAccID, amount, currency, description, kind, fee,
	).Scan(&txID); err != nil {
		return "", err
	}

	// Balanced double-entry record: debits + credits net to zero per currency.
	if err := insertLedger(ctx, tx, txID, fromAccID, currency, -amount); err != nil {
		return "", err
	}
	if err := insertLedger(ctx, tx, txID, toAccID, currency, net); err != nil {
		return "", err
	}
	if fee > 0 {
		if err := insertLedger(ctx, tx, txID, feeAccID, currency, fee); err != nil {
			return "", err
		}
	}
	return txID, nil
}

func insertLedger(ctx context.Context, tx pgx.Tx, txID, accID, currency string, amount int64) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO ledger_entries (transaction_id, account_id, currency, amount_cents) VALUES ($1, $2, $3, $4)`,
		txID, accID, currency, amount)
	return err
}

// transferToUserTx posts a P2P payment to a known recipient within an existing
// transaction (so callers can lock and update business rows atomically with the
// money move). Locks both accounts, validates balance, and applies an optional
// commission. Returns the transaction id.
func (a *App) transferToUserTx(ctx context.Context, tx pgx.Tx, senderID, recipientID, currency string, amount, fee int64, description, kind string) (string, error) {
	if recipientID == senderID {
		return "", errSelfTransfer
	}
	fromID, fromBalance, err := lockAccount(ctx, tx, senderID, currency)
	if err != nil {
		return "", errNoSenderAcct
	}
	var toID string
	if err := tx.QueryRow(ctx,
		`SELECT id FROM accounts WHERE user_id = $1 AND currency = $2 FOR UPDATE`,
		recipientID, currency,
	).Scan(&toID); err != nil {
		return "", errNoRecipient
	}
	if fromBalance < amount {
		return "", errInsufficient
	}
	return a.postPayment(ctx, tx, fromID, toID, currency, amount, fee, description, kind)
}
