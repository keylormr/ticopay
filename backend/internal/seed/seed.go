package seed

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"tuanispay/backend/internal/auth"
)

type demoUser struct {
	email      string
	fullName   string
	phone      string
	balanceCRC int64 // céntimos
	balanceUSD int64 // cents
	balanceBTC int64 // satoshis (8 dec)
	balanceETH int64 // 8 dec
	balanceUSDT int64 // 2 dec
}

var demoUsers = []demoUser{
	// ₡250 000 · $500 · 0.005 BTC · 0.1 ETH · 100 USDT
	{"maria@tuanispay.cr", "María Jiménez", "8888-0001", 25000000, 50000, 500000, 10000000, 10000},
	// ₡75 000 · $200
	{"carlos@tuanispay.cr", "Carlos Rodríguez", "8888-0002", 7500000, 20000, 0, 0, 0},
}

const demoPassword = "password123"

// Run seeds demo data (idempotent: skips if any user already exists).
func Run(ctx context.Context, pool *pgxpool.Pool) error {
	// Skip if real users already exist. The reserved system users (fees,
	// clearing, fx — migrations 0012/0016) must not count, or a fresh DB would
	// never seed (and would never get a demo admin).
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE email NOT LIKE '%@system.tuanispay'`).Scan(&count); err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if count > 0 {
		return nil
	}

	hash, err := auth.HashPassword(demoPassword)
	if err != nil {
		return err
	}

	ids := map[string]string{}
	for _, du := range demoUsers {
		var userID string
		if err := pool.QueryRow(ctx,
			`INSERT INTO users (email, phone, full_name, password_hash, kyc_status, email_verified)
			 VALUES ($1, $2, $3, $4, 'verified', true) RETURNING id`,
			du.email, du.phone, du.fullName, hash,
		).Scan(&userID); err != nil {
			return fmt.Errorf("seed user %s: %w", du.email, err)
		}
		ids[du.email] = userID
		if _, err := pool.Exec(ctx,
			`INSERT INTO accounts (user_id, currency, balance_cents) VALUES
			 ($1, 'CRC', $2), ($1, 'USD', $3), ($1, 'BTC', $4), ($1, 'ETH', $5), ($1, 'USDT', $6)`,
			userID, du.balanceCRC, du.balanceUSD, du.balanceBTC, du.balanceETH, du.balanceUSDT,
		); err != nil {
			return fmt.Errorf("seed accounts %s: %w", du.email, err)
		}
		// Remaining catalog currencies at zero balance.
		if _, err := pool.Exec(ctx,
			`INSERT INTO accounts (user_id, currency, balance_cents)
			 SELECT $1, code, 0 FROM unnest($2::text[]) AS code
			 ON CONFLICT (user_id, currency) DO NOTHING`,
			userID, []string{"USDC", "BNB", "SOL", "XRP", "ADA", "DOGE", "TRX", "DOT", "LTC", "LINK", "AVAX", "MATIC"},
		); err != nil {
			return fmt.Errorf("seed extra accounts %s: %w", du.email, err)
		}
	}

	// A demo vaquita and a pending cobro so the new features aren't empty.
	if maria, ok := ids["maria@tuanispay.cr"]; ok {
		_, _ = pool.Exec(ctx,
			`INSERT INTO pools (owner_id, name, description, goal_cents, currency)
			 VALUES ($1, 'Cumpleaños de la oficina 🎂', 'Juntemos para el queque y el regalo', 5000000, 'CRC')`,
			maria)
	}
	if maria, ok := ids["maria@tuanispay.cr"]; ok {
		if carlos, ok2 := ids["carlos@tuanispay.cr"]; ok2 {
			_, _ = pool.Exec(ctx,
				`INSERT INTO payment_requests (requester_id, target_user_id, amount_cents, currency, description)
				 VALUES ($1, $2, 1200000, 'CRC', 'Almuerzo del viernes 🌮')`,
				carlos, maria)
		}
	}

	// Promote the demo account to admin and give it a verified demo merchant so
	// the commerce and admin flows aren't empty on a fresh database.
	if maria, ok := ids["maria@tuanispay.cr"]; ok {
		_, _ = pool.Exec(ctx, `UPDATE users SET role = 'admin' WHERE id = $1`, maria)
		_, _ = pool.Exec(ctx,
			`INSERT INTO merchants (owner_id, name, category, legal_name, id_type, id_number, status, commission_bps)
			 VALUES ($1, 'Soda La Esquina', 'restaurante', 'María Jiménez', 'fisica', '102340567', 'verified', 50)`,
			maria)
	}

	fmt.Printf("[seed] created %d demo users\n", len(demoUsers))
	return nil
}
