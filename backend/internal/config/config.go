package config

import (
	"os"
	"strings"
	"time"
)

// DefaultJWTSecret is the dev-only fallback for JWT_SECRET. Production must
// override it: the server refuses to start in production with this (or any
// weak/empty) secret. See cmd/server/main.go.
const DefaultJWTSecret = "dev-secret-change-in-production-please-32+"

type Config struct {
	Port          string
	AppEnv        string // APP_ENV: "production" enables prod hardening (strong secret required, no demo seed)
	DatabaseURL   string
	JWTSecret     string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	CORSOrigins   []string
	RunMigrations bool
	SeedDemo      bool
	ResendAPIKey  string // empty → dev log sender (no real emails)
	ResendFrom    string // verified sender, e.g. "Tico Pay <no-reply@tudominio.cr>"
	EmailDebug    bool   // EMAIL_DEBUG: dev log sender prints links. Never in prod.
	AdminEmail    string // ADMIN_EMAIL: promoted to the admin role on startup (prod admin bootstrap)
}

func Load() Config {
	return Config{
		Port:          env("PORT", "8080"),
		AppEnv:        env("APP_ENV", "development"),
		DatabaseURL:   env("DATABASE_URL", "postgres://ticopay:ticopay_dev@localhost:5433/ticopay?sslmode=disable"),
		JWTSecret:     env("JWT_SECRET", DefaultJWTSecret),
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    48 * time.Hour, // short refresh window limits the value of a stolen/leaked refresh token
		CORSOrigins:   splitCSV(env("CORS_ORIGINS", "http://localhost:5174")),
		RunMigrations: env("RUN_MIGRATIONS", "true") == "true",
		SeedDemo:      env("SEED_DEMO", "true") == "true",
		ResendAPIKey:  env("RESEND_API_KEY", ""),
		ResendFrom:    env("RESEND_FROM", "onboarding@resend.dev"),
		EmailDebug:    env("EMAIL_DEBUG", "") == "true",
		AdminEmail:    env("ADMIN_EMAIL", ""),
	}
}

// IsProd reports whether APP_ENV selects production hardening.
func (c Config) IsProd() bool {
	return strings.EqualFold(c.AppEnv, "production")
}

// splitCSV parses a comma-separated env value (e.g. multiple CORS origins),
// trimming spaces and dropping empties. Falls back to the local dev origin.
func splitCSV(s string) []string {
	out := make([]string, 0, 4)
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"http://localhost:5174"}
	}
	return out
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
