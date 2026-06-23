package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pquerna/otp/totp"

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
	ver, err := a.tokenVersion(context.Background(), u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not issue tokens")
		return
	}
	access, refresh, err := a.jwt.Issue(u.ID, ver)
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
		 RETURNING id, email, COALESCE(phone,''), full_name, kyc_status, COALESCE(id_type,''), COALESCE(id_number,''), COALESCE(email_verified,false), created_at`,
		req.Email, req.Phone, req.FullName, hash,
	).Scan(&u.ID, &u.Email, &u.Phone, &u.FullName, &u.KYCStatus, &u.IDType, &u.IDNumber, &u.EmailVerified, &u.CreatedAt)
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

	// Best-effort welcome/verification email, off the request path.
	go func(uid, em, nm string) {
		bg, cancel := bgEmailCtx()
		defer cancel()
		a.sendVerificationEmail(bg, uid, em, nm)
	}(u.ID, u.Email, u.FullName)

	a.issueAuthResponse(w, http.StatusCreated, u, accounts)
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		TotpCode string `json:"totpCode"`
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
		        COALESCE(id_type,''), COALESCE(id_number,''), COALESCE(email_verified,false), created_at, password_hash
		 FROM users WHERE email = $1`, req.Email,
	).Scan(&u.ID, &u.Email, &u.Phone, &u.FullName, &u.KYCStatus, &u.IDType, &u.IDNumber, &u.EmailVerified, &u.CreatedAt, &hash)
	notFound := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !notFound {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	// Always run bcrypt — against a dummy hash when the user doesn't exist — so
	// response time doesn't reveal account existence (login timing oracle).
	if notFound {
		hash = auth.DummyHash
	}
	if !auth.CheckPassword(hash, req.Password) || notFound {
		loginAttempts.fail(req.Email)
		writeError(w, http.StatusUnauthorized, "correo o contraseña incorrectos")
		return
	}

	// Second factor: with TOTP confirmed, the password alone isn't enough.
	// 428 tells the client to ask for a code; a wrong code counts as a
	// failed attempt so codes can't be brute-forced within the window.
	enabled, secret, err := a.totpEnabled(ctx, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if enabled {
		if normalizeTotpCode(req.TotpCode) == "" {
			writeError(w, http.StatusPreconditionRequired, "se requiere el código 2FA")
			return
		}
		if !totp.Validate(normalizeTotpCode(req.TotpCode), secret) {
			loginAttempts.fail(req.Email)
			writeError(w, http.StatusUnauthorized, "código 2FA inválido")
			return
		}
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
	// Reject refresh tokens minted before a credential change (e.g. password
	// reset bumped token_version). This is what evicts a stolen session.
	ver, err := a.tokenVersion(r.Context(), claims.UserID)
	if err != nil || ver != claims.Ver {
		writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}
	access, refresh, err := a.jwt.Issue(claims.UserID, ver)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not issue tokens")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"accessToken":  access,
		"refreshToken": refresh,
	})
}
