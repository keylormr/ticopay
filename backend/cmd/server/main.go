package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ticopay/backend/internal/api"
	"ticopay/backend/internal/config"
	"ticopay/backend/internal/db"
	"ticopay/backend/internal/seed"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()
	logger := api.Logger

	// Fail closed on a weak/missing signing secret in production; warn in dev.
	if cfg.JWTSecret == "" || cfg.JWTSecret == config.DefaultJWTSecret || len(cfg.JWTSecret) < 32 {
		if cfg.IsProd() {
			logger.Error("JWT_SECRET ausente, demasiado corto (<32) o usando el default inseguro; definí uno fuerte en producción")
			os.Exit(1)
		}
		logger.Warn("JWT_SECRET inseguro/por defecto: aceptable solo en desarrollo (definí APP_ENV=production y un JWT_SECRET fuerte en prod)")
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
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           app.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("Tico Pay API listening", "port", cfg.Port)
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
