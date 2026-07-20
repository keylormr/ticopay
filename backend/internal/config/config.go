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
	AppEnv        string // APP_ENV: prod hardening unless set to an explicit dev marker (development/dev/test/local); blank or unknown → production (fail closed)
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
	c := Config{
		Port: env("PORT", "8080"),
		// Blank default (not "development") so a forgotten APP_ENV lands on the
		// fail-closed production path in IsProd(), not on dev mode. Local runs
		// must set APP_ENV=development explicitly (see .env.example / README).
		AppEnv:        env("APP_ENV", ""),
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
	// Defense in depth: a hardened (production) environment never seeds demo
	// data — which includes a public-password admin account — regardless of
	// SEED_DEMO. Together with the fail-closed IsProd(), a single forgotten
	// APP_ENV can't leave a public admin credential in a prod database.
	if c.IsProd() {
		c.SeedDemo = false
	}
	return c
}

// IsProd reports whether production hardening applies. It fails CLOSED: only a
// small set of explicit development markers disable hardening; anything else —
// an unexpected or misspelled APP_ENV like "prod" or "staging" — is treated as
// production. This prevents a typo from silently dropping the prod safeguards
// (strong-secret enforcement, no demo seed, HSTS). The default APP_ENV is
// "development" so a plain local run stays in dev; a real deploy must not leave
// APP_ENV blank or mistyped and expect hardening to switch off.
func (c Config) IsProd() bool {
	switch strings.ToLower(strings.TrimSpace(c.AppEnv)) {
	case "development", "dev", "test", "local":
		return false
	default:
		return true
	}
}

// SecretIsWeak reports whether the JWT signing secret is unset, the public dev
// default committed to the repo, or too short (<32) to be safe. A weak secret
// is fatal in production; in development the server replaces it with an
// ephemeral random one rather than ever signing with the public default.
func (c Config) SecretIsWeak() bool {
	return c.JWTSecret == "" || c.JWTSecret == DefaultJWTSecret || len(c.JWTSecret) < 32
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
