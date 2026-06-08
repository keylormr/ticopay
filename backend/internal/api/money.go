package api

import (
	"context"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"

	"ticopay/backend/internal/models"
)

// rowQuerier is satisfied by both *pgxpool.Pool and pgx.Tx.
type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

var nonDigits = regexp.MustCompile(`\D`)

// normalizePhone strips everything but digits and drops a Costa Rican country
// code prefix (506) so "+506 8888-0001" and "88880001" match.
func normalizePhone(s string) string {
	d := nonDigits.ReplaceAllString(s, "")
	if len(d) == 11 && strings.HasPrefix(d, "506") {
		d = d[3:]
	}
	return d
}

func validCurrency(c string) bool {
	return c == "CRC" || c == "USD"
}

// parseRecipient classifies a "to" value as an email or a phone number.
func parseRecipient(to string) (email, phoneNorm string) {
	to = strings.TrimSpace(to)
	if strings.Contains(to, "@") {
		return strings.ToLower(to), ""
	}
	return "", normalizePhone(to)
}

// resolveRecipientAccount finds the account (of the given currency) belonging to
// the user identified by an email or phone number.
func resolveRecipientAccount(ctx context.Context, q rowQuerier, to, currency string) (accID, userID, name string, err error) {
	email, phone := parseRecipient(to)
	err = q.QueryRow(ctx, `
		SELECT acc.id, u.id, u.full_name
		FROM users u
		JOIN accounts acc ON acc.user_id = u.id AND acc.currency = $3
		WHERE ($1 <> '' AND lower(u.email) = $1)
		   OR ($2 <> '' AND regexp_replace(COALESCE(u.phone,''), '\D', '', 'g') = $2)
		LIMIT 1`,
		email, phone, currency,
	).Scan(&accID, &userID, &name)
	return accID, userID, name, err
}

func (a *App) fetchAccounts(ctx context.Context, uid string) ([]models.Account, error) {
	rows, err := a.pool.Query(ctx,
		`SELECT id, currency, balance_cents FROM accounts WHERE user_id = $1 ORDER BY currency`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accounts := make([]models.Account, 0, 2)
	for rows.Next() {
		var acc models.Account
		if err := rows.Scan(&acc.ID, &acc.Currency, &acc.BalanceCents); err != nil {
			return nil, err
		}
		accounts = append(accounts, acc)
	}
	return accounts, rows.Err()
}
