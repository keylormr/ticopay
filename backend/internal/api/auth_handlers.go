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
	AccessToken  string           `json:"accessToken"`
	RefreshToken string           `json:"refreshToken"`
	User         models.User      `json:"user"`
	Accounts     []models.Account `json:"accounts"`
}

func (a *App) issueAuthResponse(w http.ResponseWriter, status int, u models.User, accounts []models.Account) {
	access, refresh, err := a.jwt.Issue(u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not issue tokens")
		return
	}
	writeJSON(w, status, authResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		User:         u,
		Accounts:     accounts,
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
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.FullName = strings.TrimSpace(req.FullName)
	req.Phone = strings.TrimSpace(req.Phone)
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		writeError(w, http.StatusBadRequest, "ingresá un correo válido")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "la contraseña debe tener al menos 8 caracteres")
		return
	}
	if req.FullName == "" {
		writeError(w, http.StatusBadRequest, "el nombre completo es obligatorio")
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
		 RETURNING id, email, COALESCE(phone,''), full_name, kyc_status, COALESCE(id_type,''), COALESCE(id_number,''), created_at`,
		req.Email, req.Phone, req.FullName, hash,
	).Scan(&u.ID, &u.Email, &u.Phone, &u.FullName, &u.KYCStatus, &u.IDType, &u.IDNumber, &u.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "ese correo o teléfono ya está registrado")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not create user")
		return
	}

	// Every user gets a wallet for each supported currency (fiat + crypto).
	if _, err := tx.Exec(ctx,
		`INSERT INTO accounts (user_id, currency, balance_cents)
		 SELECT $1, code, 0 FROM unnest($2::text[]) AS code`,
		u.ID, allCurrencyCodes(),
	); err != nil {
		writeError(w, http.StatusInternalServerError, "could not create accounts")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "could not complete registration")
		return
	}

	accounts, err := a.fetchAccounts(ctx, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load accounts")
		return
	}
	a.issueAuthResponse(w, http.StatusCreated, u, accounts)
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if loginAttempts.locked(req.Email) {
		writeError(w, http.StatusTooManyRequests, "demasiados intentos, probá de nuevo en unos minutos")
		return
	}

	ctx := r.Context()
	var (
		u    models.User
		hash string
	)
	err := a.pool.QueryRow(ctx,
		`SELECT id, email, COALESCE(phone,''), full_name, kyc_status,
		        COALESCE(id_type,''), COALESCE(id_number,''), created_at, password_hash
		 FROM users WHERE email = $1`, req.Email,
	).Scan(&u.ID, &u.Email, &u.Phone, &u.FullName, &u.KYCStatus, &u.IDType, &u.IDNumber, &u.CreatedAt, &hash)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !auth.CheckPassword(hash, req.Password)) {
		loginAttempts.fail(req.Email)
		writeError(w, http.StatusUnauthorized, "correo o contraseña incorrectos")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	accounts, err := a.fetchAccounts(ctx, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load accounts")
		return
	}
	loginAttempts.reset(req.Email)
	a.issueAuthResponse(w, http.StatusOK, u, accounts)
}

func (a *App) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
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
