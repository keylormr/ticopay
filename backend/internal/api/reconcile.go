package api

import (
	"context"
	"time"
)

// ledgerDrift is one account whose cached balance disagrees with the journal:
// balance_cents != opening_offset_cents + SUM(ledger_entries). A non-empty
// reconciliation means some code path moved a balance without a matching ledger
// entry — a bug, since the journal (plus the recorded opening offset) is the
// source of truth.
type ledgerDrift struct {
	AccountID  string `json:"accountId"`
	UserName   string `json:"userName"`
	Currency   string `json:"currency"`
	DriftCents int64  `json:"driftCents"`
}

// reconcileLedger returns every account whose balance drifts from the journal,
// worst first. An empty slice means the ledger is fully reconciled.
func (a *App) reconcileLedger(ctx context.Context) ([]ledgerDrift, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT a.id, u.full_name, a.currency,
		       a.balance_cents - a.opening_offset_cents - COALESCE(SUM(le.amount_cents), 0) AS drift
		FROM accounts a
		JOIN users u ON u.id = a.user_id
		LEFT JOIN ledger_entries le ON le.account_id = a.id
		GROUP BY a.id, u.full_name, a.currency, a.balance_cents, a.opening_offset_cents
		HAVING a.balance_cents - a.opening_offset_cents - COALESCE(SUM(le.amount_cents), 0) <> 0
		ORDER BY abs(a.balance_cents - a.opening_offset_cents - COALESCE(SUM(le.amount_cents), 0)) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ledgerDrift, 0)
	for rows.Next() {
		var d ledgerDrift
		if err := rows.Scan(&d.AccountID, &d.UserName, &d.Currency, &d.DriftCents); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// totalAbsDrift sums the absolute drift across accounts.
func totalAbsDrift(drifts []ledgerDrift) int64 {
	var total int64
	for _, d := range drifts {
		if d.DriftCents < 0 {
			total -= d.DriftCents
		} else {
			total += d.DriftCents
		}
	}
	return total
}

// ReconcileLoop periodically checks the ledger invariant, publishes the total
// absolute drift as the tuanispay_ledger_drift_cents metric, and logs any drift
// as a high-severity event (it should always be zero; a non-zero value is a
// money bug to investigate). It runs a check immediately, then every `every`
// until ctx is cancelled — safe to launch as a goroutine at startup.
func (a *App) ReconcileLoop(ctx context.Context, every time.Duration) {
	check := func() {
		// Derive from the loop ctx so an app shutdown cancels an in-flight check.
		c, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		drifts, err := a.reconcileLedger(c)
		if err != nil {
			Logger.Error("ledger reconcile failed", "error", err)
			return
		}
		total := totalAbsDrift(drifts)
		metrics.ledgerDriftCents.Store(total)
		if len(drifts) > 0 {
			Logger.Error("ledger drift detected",
				"accounts", len(drifts), "totalAbsDriftCents", total, "worst", drifts[0])
		}
	}

	check()
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check()
		}
	}
}
