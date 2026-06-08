package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"ticopay/backend/internal/auth"
	"ticopay/backend/internal/models"
)

type authResponse struct {
	AccessToken  string         `json:"accessToken"`
	RefreshToken string         `json:"refreshToken"`
	User         models.User    `json:"user"`
	Account      models.Account `json:"account"`
}

func (a *App) issueAuthResponse(w http.ResponseWriter, status int, u models.User, acc models.Account) {
	access, refresh, err := a.jwt.Issue(u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not issue tokens")
		return
	}
	writeJSON(w, status, authResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		User:         u,
		Account:      acc,
	})
}

func (a *App) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		FullName string `json:"fullName"`
		Phone    string `json:"phone"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.FullName = strings.TrimSpace(req.FullName)
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		writeError(w, http.StatusBadRequest, "a valid email is required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	if req.FullName == "" {
		writeError(w, http.StatusBadRequest, "full name is required")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not hash password")
		return
	}

	ctx := r.Context()
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer tx.Rollback(ctx)

	var u models.User
	err = tx.QueryRow(ctx,
		`INSERT INTO users (email, phone, full_name, password_hash)
		 VALUES ($1, NULLIF($2,''), $3, $4)
		 RETURNING id, email, COALESCE(phone,''), full_name, created_at`,
		req.Email, req.Phone, req.FullName, hash,
	).Scan(&u.ID, &u.Email, &u.Phone, &u.FullName, &u.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "email already registered")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not create user")
		return
	}

	var acc models.Account
	acc.Currency = "CRC"
	err = tx.QueryRow(ctx,
		`INSERT INTO accounts (user_id, currency, balance_cents)
		 VALUES ($1, 'CRC', 0)
		 RETURNING id, currency, balance_cents`,
		u.ID,
	).Scan(&acc.ID, &acc.Currency, &acc.BalanceCents)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create account")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "could not complete registration")
		return
	}

	a.issueAuthResponse(w, http.StatusCreated, u, acc)
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	ctx := r.Context()
	var (
		u    models.User
		hash string
	)
	err := a.pool.QueryRow(ctx,
		`SELECT id, email, COALESCE(phone,''), full_name, created_at, password_hash
		 FROM users WHERE email = $1`, req.Email,
	).Scan(&u.ID, &u.Email, &u.Phone, &u.FullName, &u.CreatedAt, &hash)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !auth.CheckPassword(hash, req.Password)) {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	acc, err := a.fetchAccount(ctx, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load account")
		return
	}
	a.issueAuthResponse(w, http.StatusOK, u, acc)
}

func (a *App) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	claims, err := a.jwt.Parse(req.RefreshToken, "refresh")
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}
	access, refresh, err := a.jwt.Issue(claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not issue tokens")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"accessToken":  access,
		"refreshToken": refresh,
	})
}
