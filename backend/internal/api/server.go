package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5/pgxpool"

	"ticopay/backend/internal/auth"
	"ticopay/backend/internal/config"
)

type App struct {
	pool *pgxpool.Pool
	jwt  *auth.Manager
	cfg  config.Config
	wa   *webauthn.WebAuthn
}

func NewApp(pool *pgxpool.Pool, cfg config.Config) *App {
	app := &App{
		pool: pool,
		jwt:  auth.NewManager(cfg.JWTSecret, cfg.AccessTTL, cfg.RefreshTTL),
		cfg:  cfg,
	}
	app.wa = newWebAuthn(cfg)
	return app
}

func (a *App) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   a.cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Lang"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Innermost middleware: tag the writer with the request language so error
	// messages can be localized.
	r.Use(withLang)

	r.Get("/health", a.handleHealth)

	r.Route("/api", func(r chi.Router) {
		r.Post("/auth/register", a.handleRegister)
		r.Post("/auth/login", a.handleLogin)
		r.Post("/auth/refresh", a.handleRefresh)

		// Passwordless login with a passkey (no auth yet).
		r.Post("/auth/passkey/begin", a.handlePasskeyLoginBegin)
		r.Post("/auth/passkey/finish", a.handlePasskeyLoginFinish)

		// Public: USD/CRC reference rate + full fiat/crypto rate table.
		r.Get("/exchange-rate", a.handleExchangeRate)
		r.Get("/rates", a.handleRates)

		r.Group(func(r chi.Router) {
			r.Use(a.requireAuth)

			r.Get("/me", a.handleMe)
			r.Get("/transactions", a.handleListTransactions)
			r.Post("/transactions", a.handleSendMoney)
			r.Post("/sinpe", a.handleSinpe)
			r.Post("/convert", a.handleConvert)
			r.Post("/kyc", a.handleSubmitKYC)

			r.Post("/requests", a.handleCreateRequest)
			r.Get("/requests", a.handleListRequests)
			r.Get("/requests/{id}", a.handleGetRequest)
			r.Post("/requests/{id}/pay", a.handlePayRequest)

			r.Post("/pools", a.handleCreatePool)
			r.Get("/pools", a.handleListPools)
			r.Get("/pools/{id}", a.handleGetPool)
			r.Post("/pools/{id}/contribute", a.handleContributePool)

			r.Get("/billers", a.handleListBillers)
			r.Post("/payments/service", a.handlePayService)

			// Passkey management (requires an active session).
			r.Post("/passkeys/register/begin", a.handlePasskeyRegisterBegin)
			r.Post("/passkeys/register/finish", a.handlePasskeyRegisterFinish)
			r.Get("/passkeys", a.handleListPasskeys)
			r.Delete("/passkeys/{id}", a.handleDeletePasskey)
		})
	})

	return r
}

// --- JSON helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": localizeError(w, msg)})
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := "ok"
	if err := a.pool.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded", "db": "down"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status, "db": "up"})
}
