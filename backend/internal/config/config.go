package config

import (
	"os"
	"strings"
	"time"
)

type Config struct {
	Port          string
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
}

func Load() Config {
	return Config{
		Port:          env("PORT", "8080"),
		DatabaseURL:   env("DATABASE_URL", "postgres://ticopay:ticopay_dev@localhost:5433/ticopay?sslmode=disable"),
		JWTSecret:     env("JWT_SECRET", "dev-secret-change-in-production-please-32+"),
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    7 * 24 * time.Hour,
		CORSOrigins:   splitCSV(env("CORS_ORIGINS", "http://localhost:5174")),
		RunMigrations: env("RUN_MIGRATIONS", "true") == "true",
		SeedDemo:      env("SEED_DEMO", "true") == "true",
		ResendAPIKey:  env("RESEND_API_KEY", ""),
		ResendFrom:    env("RESEND_FROM", "onboarding@resend.dev"),
		EmailDebug:    env("EMAIL_DEBUG", "") == "true",
	}
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
