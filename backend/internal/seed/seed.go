package seed

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"ticopay/backend/internal/auth"
)

type demoUser struct {
	email    string
	fullName string
	phone    string
	balance  int64 // céntimos
}

var demoUsers = []demoUser{
	{"maria@ticopay.cr", "María Jiménez", "8888-0001", 25000000},  // ₡250 000,00
	{"carlos@ticopay.cr", "Carlos Rodríguez", "8888-0002", 7500000}, // ₡75 000,00
}

const demoPassword = "password123"

// Run seeds demo users (idempotent: skips if any user already exists).
func Run(ctx context.Context, pool *pgxpool.Pool) error {
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if count > 0 {
		return nil
	}

	hash, err := auth.HashPassword(demoPassword)
	if err != nil {
		return err
	}

	for _, du := range demoUsers {
		var userID string
		if err := pool.QueryRow(ctx,
			`INSERT INTO users (email, phone, full_name, password_hash)
			 VALUES ($1, $2, $3, $4) RETURNING id`,
			du.email, du.phone, du.fullName, hash,
		).Scan(&userID); err != nil {
			return fmt.Errorf("seed user %s: %w", du.email, err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO accounts (user_id, currency, balance_cents) VALUES ($1, 'CRC', $2)`,
			userID, du.balance,
		); err != nil {
			return fmt.Errorf("seed account %s: %w", du.email, err)
		}
	}

	fmt.Printf("[seed] created %d demo users (password: %s)\n", len(demoUsers), demoPassword)
	return nil
}
