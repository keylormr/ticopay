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
	if cfg.SeedDemo {
		if err := seed.Run(ctx, pool); err != nil {
			logger.Error("seed failed", "error", err)
			os.Exit(1)
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
