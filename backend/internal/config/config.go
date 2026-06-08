package config

import (
	"os"
	"time"
)

type Config struct {
	Port           string
	DatabaseURL    string
	JWTSecret      string
	AccessTTL      time.Duration
	RefreshTTL     time.Duration
	CORSOrigins    []string
	RunMigrations  bool
	SeedDemo       bool
}

func Load() Config {
	return Config{
		Port:          env("PORT", "8080"),
		DatabaseURL:   env("DATABASE_URL", "postgres://ticopay:ticopay_dev@localhost:5433/ticopay?sslmode=disable"),
		JWTSecret:     env("JWT_SECRET", "dev-secret-change-in-production-please-32+"),
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    7 * 24 * time.Hour,
		CORSOrigins:   []string{env("CORS_ORIGINS", "http://localhost:5174")},
		RunMigrations: env("RUN_MIGRATIONS", "true") == "true",
		SeedDemo:      env("SEED_DEMO", "true") == "true",
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
