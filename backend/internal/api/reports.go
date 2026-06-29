package api

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var errBadDate = errors.New("bad date")

// sanitizeCSVCell neutralizes spreadsheet formula injection: a cell beginning
// with =, +, -, @ (or TAB/CR) is treated as a formula by Excel/Sheets, so we
// prefix it with an apostrophe to force text. Applied to user-controlled text
// columns of the CSV export.
func sanitizeCSVCell(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}

// scanCountMap runs a `SELECT key_text, count_bigint FROM ...` query and returns
// it as a map plus the grand total.
func (a *App) scanCountMap(ctx context.Context, sql string, args ...any) (map[string]int64, int64, error) {
	rows, err := a.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := map[string]int64{}
	var total int64
	for rows.Next() {
		var k string
		var n int64
		if err := rows.Scan(&k, &n); err != nil {
			return nil, 0, err
		}
		out[k] = n
		total += n
	}
	return out, total, rows.Err()
}

type currencyAmount struct {
	Currency    string `json:"currency"`
	AmountCents int64  `json:"amountCents"`
	Count       int64  `json:"count,omitempty"`
}

// handleReportOverview returns the headline KPIs for the platform.
func (a *App) handleReportOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	byRole, totalUsers, err := a.scanCountMap(ctx,
		`SELECT role, count(*) FROM users WHERE `+systemEmailFilter+` GROUP BY role`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load users")
		return
	}

	var active30d, disabled int64
	if err := a.pool.QueryRow(ctx,
		`SELECT count(DISTINCT a.user_id)
		 FROM transactions t
		 JOIN accounts a ON a.id = t.from_account_id OR a.id = t.to_account_id
		 JOIN users u ON u.id = a.user_id
		 WHERE t.created_at >= now() - interval '30 days' AND lower(u.email) NOT LIKE '%@system.ticopay'`,
	).Scan(&active30d); err != nil {
		writeError(w, http.StatusInternalServerError, "could not load active users")
		return
	}
	if err := a.pool.QueryRow(ctx,
		`SELECT count(*) FROM users WHERE disabled AND `+systemEmailFilter).Scan(&disabled); err != nil {
		writeError(w, http.StatusInternalServerError, "could not load users")
		return
	}

	merchantsByStatus, _, err := a.scanCountMap(ctx, `SELECT status, count(*) FROM merchants GROUP BY status`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load merchants")
		return
	}

	byKind, totalTx, err := a.scanCountMap(ctx, `SELECT kind, count(*) FROM transactions GROUP BY kind`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load transactions")
		return
	}

	volume, err := a.scanCurrencyAmounts(ctx,
		`SELECT currency, COALESCE(sum(amount_cents),0), count(*) FROM transactions GROUP BY currency ORDER BY 2 DESC`, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load volume")
		return
	}
	fees, err := a.scanCurrencyAmounts(ctx,
		`SELECT currency, COALESCE(sum(fee_cents),0), 0 FROM transactions WHERE fee_cents > 0 GROUP BY currency ORDER BY 2 DESC`, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load fees")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"users": map[string]any{
			"total": totalUsers, "byRole": byRole, "active30d": active30d, "disabled": disabled,
		},
		"merchants":        merchantsByStatus,
		"transactions":     map[string]any{"total": totalTx, "byKind": byKind},
		"volumeByCurrency": volume,
		"feesByCurrency":   fees,
	})
}

func (a *App) scanCurrencyAmounts(ctx context.Context, sql string, withCount bool, args ...any) ([]currencyAmount, error) {
	rows, err := a.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]currencyAmount, 0)
	for rows.Next() {
		var c currencyAmount
		var cnt int64
		if err := rows.Scan(&c.Currency, &c.AmountCents, &cnt); err != nil {
			return nil, err
		}
		if withCount {
			c.Count = cnt
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// handleReportTimeseries returns a continuous daily series (gaps filled with 0)
// for a metric over the last N days, optionally filtered by currency.
func (a *App) handleReportTimeseries(w http.ResponseWriter, r *http.Request) {
	days := qInt(r, "days", 30, 1, 365)
	currency := strings.TrimSpace(r.URL.Query().Get("currency"))
	if currency != "" && !validCurrency(currency) {
		writeError(w, http.StatusBadRequest, "moneda no soportada")
		return
	}

	metricExpr := map[string]string{
		"count":  "count(*)",
		"volume": "COALESCE(sum(amount_cents),0)",
		"fees":   "COALESCE(sum(fee_cents),0)",
	}[strings.TrimSpace(r.URL.Query().Get("metric"))]
	if metricExpr == "" {
		metricExpr = "count(*)" // default
	}

	rows, err := a.pool.Query(r.Context(),
		`SELECT d::date AS day, COALESCE(agg.v, 0) AS value
		 FROM generate_series((now() - ($1::int - 1) * interval '1 day')::date, now()::date, interval '1 day') d
		 LEFT JOIN (
		     SELECT created_at::date AS day, `+metricExpr+` AS v
		     FROM transactions
		     WHERE created_at >= (now() - ($1::int - 1) * interval '1 day')::date
		       AND ($2 = '' OR currency = $2)
		     GROUP BY 1
		 ) agg ON agg.day = d::date
		 ORDER BY day`, days, currency)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load series")
		return
	}
	defer rows.Close()

	type point struct {
		Date  string `json:"date"`
		Value int64  `json:"value"`
	}
	series := make([]point, 0, days)
	for rows.Next() {
		var day time.Time
		var v int64
		if err := rows.Scan(&day, &v); err != nil {
			writeError(w, http.StatusInternalServerError, "could not read series")
			return
		}
		series = append(series, point{Date: day.Format("2006-01-02"), Value: v})
	}
	writeJSON(w, http.StatusOK, map[string]any{"series": series})
}

// handleReportByKind breaks transactions down by kind over the last N days.
func (a *App) handleReportByKind(w http.ResponseWriter, r *http.Request) {
	days := qInt(r, "days", 30, 1, 365)
	rows, err := a.pool.Query(r.Context(),
		`SELECT kind, count(*), COALESCE(sum(amount_cents),0)
		 FROM transactions
		 WHERE created_at >= (now() - ($1::int - 1) * interval '1 day')::date
		 GROUP BY kind ORDER BY count(*) DESC`, days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load breakdown")
		return
	}
	defer rows.Close()
	type kindRow struct {
		Kind        string `json:"kind"`
		Count       int64  `json:"count"`
		AmountCents int64  `json:"amountCents"`
	}
	out := make([]kindRow, 0)
	for rows.Next() {
		var k kindRow
		if err := rows.Scan(&k.Kind, &k.Count, &k.AmountCents); err != nil {
			writeError(w, http.StatusInternalServerError, "could not read breakdown")
			return
		}
		out = append(out, k)
	}
	writeJSON(w, http.StatusOK, map[string]any{"byKind": out})
}

// handleReportLedgerHealth surfaces the double-entry invariant (net per currency
// should be 0) and the system account balances.
func (a *App) handleReportLedgerHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	netRows, err := a.pool.Query(ctx,
		`SELECT currency, COALESCE(sum(amount_cents),0) FROM ledger_entries GROUP BY currency ORDER BY currency`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load ledger")
		return
	}
	defer netRows.Close()
	type netRow struct {
		Currency string `json:"currency"`
		NetCents int64  `json:"netCents"`
	}
	net := make([]netRow, 0)
	balanced := true
	for netRows.Next() {
		var n netRow
		if err := netRows.Scan(&n.Currency, &n.NetCents); err != nil {
			writeError(w, http.StatusInternalServerError, "could not read ledger")
			return
		}
		if n.NetCents != 0 {
			balanced = false
		}
		net = append(net, n)
	}

	sysRows, err := a.pool.Query(ctx,
		`SELECT u.full_name, a.currency, a.balance_cents
		 FROM accounts a JOIN users u ON u.id = a.user_id
		 WHERE a.is_system ORDER BY u.full_name, a.currency`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load system accounts")
		return
	}
	defer sysRows.Close()
	type sysRow struct {
		Account     string `json:"account"`
		Currency    string `json:"currency"`
		BalanceCents int64 `json:"balanceCents"`
	}
	sys := make([]sysRow, 0)
	for sysRows.Next() {
		var s sysRow
		if err := sysRows.Scan(&s.Account, &s.Currency, &s.BalanceCents); err != nil {
			writeError(w, http.StatusInternalServerError, "could not read system accounts")
			return
		}
		sys = append(sys, s)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"balanced": balanced, "netByCurrency": net, "systemAccounts": sys,
	})
}

// txFilter builds a parameterized WHERE clause from the query string filters
// (from, to, kind, currency, q) shared by the drill-down list and CSV export.
func txFilter(r *http.Request) (string, []any, error) {
	var conds []string
	var args []any
	add := func(expr, val string) {
		args = append(args, val)
		conds = append(conds, fmt.Sprintf(expr, len(args)))
	}
	// Validate dates in Go so a bad value is a 400, not a Postgres cast 500.
	if v := strings.TrimSpace(r.URL.Query().Get("from")); v != "" {
		if _, err := time.Parse("2006-01-02", v); err != nil {
			return "", nil, errBadDate
		}
		add("t.created_at >= $%d::date", v)
	}
	if v := strings.TrimSpace(r.URL.Query().Get("to")); v != "" {
		if _, err := time.Parse("2006-01-02", v); err != nil {
			return "", nil, errBadDate
		}
		add("t.created_at < ($%d::date + 1)", v)
	}
	if v := strings.TrimSpace(r.URL.Query().Get("kind")); v != "" {
		add("t.kind = $%d", v)
	}
	if v := strings.TrimSpace(r.URL.Query().Get("currency")); v != "" {
		add("t.currency = $%d", v)
	}
	if v := strings.TrimSpace(r.URL.Query().Get("q")); v != "" {
		add("t.description ILIKE '%%'||$%d||'%%'", v)
	}
	if len(conds) == 0 {
		return "", args, nil
	}
	return "WHERE " + strings.Join(conds, " AND "), args, nil
}

const txSelect = `SELECT t.id, t.created_at, t.kind, t.status, t.currency, t.amount_cents, t.fee_cents,
       COALESCE(fu.email, ''), COALESCE(tu.email, ''), t.description
 FROM transactions t
 LEFT JOIN accounts fa ON fa.id = t.from_account_id
 LEFT JOIN accounts ta ON ta.id = t.to_account_id
 LEFT JOIN users fu ON fu.id = fa.user_id
 LEFT JOIN users tu ON tu.id = ta.user_id`

type txReportRow struct {
	ID          string    `json:"id"`
	CreatedAt   time.Time `json:"createdAt"`
	Kind        string    `json:"kind"`
	Status      string    `json:"status"`
	Currency    string    `json:"currency"`
	AmountCents int64     `json:"amountCents"`
	FeeCents    int64     `json:"feeCents"`
	FromEmail   string    `json:"fromEmail"`
	ToEmail     string    `json:"toEmail"`
	Description string    `json:"description"`
}

func scanTxRows(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]txReportRow, error) {
	out := make([]txReportRow, 0)
	for rows.Next() {
		var t txReportRow
		if err := rows.Scan(&t.ID, &t.CreatedAt, &t.Kind, &t.Status, &t.Currency,
			&t.AmountCents, &t.FeeCents, &t.FromEmail, &t.ToEmail, &t.Description); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// handleReportTransactions is the filterable, paginated drill-down.
func (a *App) handleReportTransactions(w http.ResponseWriter, r *http.Request) {
	where, args, err := txFilter(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "fecha inválida (usá AAAA-MM-DD)")
		return
	}
	limit := qInt(r, "limit", 50, 1, 500)
	offset := qInt(r, "offset", 0, 0, 5_000_000)

	var total int64
	if err := a.pool.QueryRow(r.Context(),
		`SELECT count(*) FROM transactions t `+where, args...).Scan(&total); err != nil {
		writeError(w, http.StatusInternalServerError, "could not count transactions")
		return
	}

	args = append(args, limit, offset)
	rows, err := a.pool.Query(r.Context(),
		txSelect+" "+where+fmt.Sprintf(" ORDER BY t.created_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load transactions")
		return
	}
	defer rows.Close()
	list, err := scanTxRows(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read transactions")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"transactions": list, "total": total, "limit": limit, "offset": offset})
}

// handleReportTransactionsCSV streams the filtered transactions as CSV (capped).
func (a *App) handleReportTransactionsCSV(w http.ResponseWriter, r *http.Request) {
	where, args, err := txFilter(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "fecha inválida (usá AAAA-MM-DD)")
		return
	}
	const maxRows = 10000
	args = append(args, maxRows)
	rows, err := a.pool.Query(r.Context(),
		txSelect+" "+where+fmt.Sprintf(" ORDER BY t.created_at DESC LIMIT $%d", len(args)), args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load transactions")
		return
	}
	defer rows.Close()
	list, err := scanTxRows(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read transactions")
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=transacciones.csv")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"id", "fecha", "tipo", "estado", "moneda", "monto_cents", "comision_cents", "de", "para", "detalle"})
	for _, t := range list {
		// Sanitize user-controlled text columns against spreadsheet formula
		// injection; numeric/date columns are machine-formatted and safe.
		_ = cw.Write([]string{
			sanitizeCSVCell(t.ID), t.CreatedAt.Format(time.RFC3339), sanitizeCSVCell(t.Kind), sanitizeCSVCell(t.Status),
			sanitizeCSVCell(t.Currency), strconv.FormatInt(t.AmountCents, 10), strconv.FormatInt(t.FeeCents, 10),
			sanitizeCSVCell(t.FromEmail), sanitizeCSVCell(t.ToEmail), sanitizeCSVCell(t.Description),
		})
	}
	cw.Flush()
}
