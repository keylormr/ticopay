package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tuanispay/backend/internal/api"
	"tuanispay/backend/internal/config"
	"tuanispay/backend/internal/db"
	"tuanispay/backend/internal/seed"
)

// randomSecret returns a fresh base64 secret for signing JWTs in development,
// so the server never signs with the public default committed to the repo.
func randomSecret() (string, error) {
	b := make([]byte, 48)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func main() {
	cfg := config.Load()
	ctx := context.Background()
	logger := api.Logger

	// A weak, missing, or default signing secret is fatal in production. In
	// development we never sign with the public default committed to the repo
	// (anyone could forge tokens with it) — generate an ephemeral random secret
	// instead. IsProd() fails closed, so a missing or misspelled APP_ENV lands
	// on the strict production path.
	if cfg.SecretIsWeak() {
		if cfg.IsProd() {
			logger.Error("JWT_SECRET ausente, débil (<32) o el default público del repo; definí uno fuerte (producción) o APP_ENV=development para desarrollo local")
			os.Exit(1)
		}
		secret, err := randomSecret()
		if err != nil {
			logger.Error("no se pudo generar un secreto de desarrollo", "error", err)
			os.Exit(1)
		}
		cfg.JWTSecret = secret
		logger.Warn("JWT_SECRET inseguro o ausente: usando un secreto EFÍMERO aleatorio para desarrollo; definí JWT_SECRET para que las sesiones persistan entre reinicios")
	}

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connect failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if cfg.RunMigrations {
		if err := db.Migrate(ctx, pool); err != nil {
			logger.Error("migrate failed", "error", err)
			os.Exit(1)
		}
	}
	// Never seed demo data (which includes a public-password admin) in
	// production, regardless of SEED_DEMO.
	if cfg.SeedDemo && !cfg.IsProd() {
		if err := seed.Run(ctx, pool); err != nil {
			logger.Error("seed failed", "error", err)
			os.Exit(1)
		}
	}
	// Production admin bootstrap: promote a real account named by ADMIN_EMAIL.
	// This avoids granting admin to the public demo credential.
	if cfg.AdminEmail != "" {
		if _, err := pool.Exec(ctx,
			`UPDATE users SET role = 'admin' WHERE lower(email) = lower($1)`, cfg.AdminEmail); err != nil {
			logger.Error("admin promotion failed", "email", cfg.AdminEmail, "error", err)
		} else {
			logger.Info("admin role ensured", "email", cfg.AdminEmail)
		}
	}

	app := api.NewApp(pool, cfg)

	// Periodically purge terminal idempotency keys so the table doesn't grow
	// without bound (a key is only needed within a client's retry window).
	go app.ReapIdempotencyKeys(ctx, 6*time.Hour)

	// Continuously reconcile cached balances against the journal, publishing the
	// drift metric and alerting on any divergence (which should always be zero).
	go app.ReconcileLoop(ctx, 5*time.Minute)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           app.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("TuanisPay API listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
	}
}
