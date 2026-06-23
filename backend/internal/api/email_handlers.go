package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"ticopay/backend/internal/auth"
)

const (
	resetTokenTTL  = 30 * time.Minute
	verifyTokenTTL = 24 * time.Hour
)

// genToken returns a high-entropy URL-safe token and its SHA-256 hex hash.
// Only the hash is ever stored, so a DB leak can't be replayed.
func genToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// appURL is the frontend base used to build email links (same origin we allow
// via CORS). Falls back to localhost dev port.
func (a *App) appURL() string {
	if len(a.cfg.CORSOrigins) > 0 && a.cfg.CORSOrigins[0] != "" {
		return strings.TrimRight(a.cfg.CORSOrigins[0], "/")
	}
	return "http://localhost:5174"
}

// --- password reset ---

func (a *App) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))

	// Anti-enumeration: respond 200 identically whether or not the email exists.
	var uid, name string
	err := a.pool.QueryRow(r.Context(), `SELECT id, full_name FROM users WHERE email = $1`, email).Scan(&uid, &name)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if err == nil {
		// Token minting + the blocking email send run off the request path so
		// response time doesn't leak account existence (timing oracle).
		go a.issueResetToken(uid, name, email)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// bgEmailCtx is a detached, time-bounded context for fire-and-forget sends so a
// slow mail provider can never hang a request or leak goroutines forever.
func bgEmailCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 15*time.Second)
}

func (a *App) issueResetToken(uid, name, email string) {
	ctx, cancel := bgEmailCtx()
	defer cancel()
	raw, hash, err := genToken()
	if err != nil {
		return
	}
	// Supersede prior unused links (per-account throttle + avoids token pile-up).
	_, _ = a.pool.Exec(ctx,
		`UPDATE password_reset_tokens SET used_at = now() WHERE user_id = $1 AND used_at IS NULL`, uid)
	if _, err := a.pool.Exec(ctx,
		`INSERT INTO password_reset_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		uid, hash, time.Now().Add(resetTokenTTL),
	); err != nil {
		return
	}
	a.sendResetEmail(ctx, email, name, raw)
}

func (a *App) sendResetEmail(ctx context.Context, to, name, rawToken string) {
	link := fmt.Sprintf("%s/reset?token=%s", a.appURL(), rawToken)
	subject := "Restablecé tu contraseña de Tico Pay"
	body := fmt.Sprintf(
		`<div style="font-family:system-ui,sans-serif;max-width:480px">
		  <h2>Hola %s 👋</h2>
		  <p>Pediste restablecer tu contraseña de <strong>Tico Pay</strong>. Hacé clic en el botón (el enlace vence en 30 minutos):</p>
		  <p><a href="%s" style="background:#002b7f;color:#fff;padding:12px 20px;border-radius:8px;text-decoration:none;display:inline-block">Cambiar mi contraseña</a></p>
		  <p style="color:#64748b;font-size:13px">Si no fuiste vos, ignorá este correo: tu contraseña no cambia.</p>
		</div>`,
		html.EscapeString(name), link)
	_ = a.mailer.Send(ctx, to, subject, body)
}

func (a *App) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "la contraseña debe tener al menos 8 caracteres")
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		writeError(w, http.StatusBadRequest, "enlace inválido o expirado")
		return
	}
	ctx := r.Context()

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not hash password")
		return
	}

	tx, err := a.pool.Begin(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer tx.Rollback(ctx)

	// Atomically claim the token: the row-level guard (used_at IS NULL) makes
	// consumption single-use even under concurrent requests with the same link.
	var uid string
	err = tx.QueryRow(ctx,
		`UPDATE password_reset_tokens SET used_at = now()
		 WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
		 RETURNING user_id`,
		hashToken(token),
	).Scan(&uid)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusBadRequest, "enlace inválido o expirado")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Invalidate any other outstanding reset links for this user.
	if _, err := tx.Exec(ctx,
		`UPDATE password_reset_tokens SET used_at = now() WHERE user_id = $1 AND used_at IS NULL`, uid,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo completar la operación")
		return
	}
	// Change the password, mark the email verified (the link proved ownership),
	// and bump token_version to evict every existing session — all atomically.
	if _, err := tx.Exec(ctx,
		`UPDATE users SET password_hash = $1, email_verified = true, token_version = token_version + 1 WHERE id = $2`,
		hash, uid,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo completar la operación")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo completar la operación")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- email verification ---

// sendVerificationEmail mints a verification token and emails the link.
// Best-effort: callers ignore the error (logged by the sender).
func (a *App) sendVerificationEmail(ctx context.Context, uid, to, name string) {
	raw, hash, err := genToken()
	if err != nil {
		return
	}
	if _, err := a.pool.Exec(ctx,
		`INSERT INTO email_verification_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		uid, hash, time.Now().Add(verifyTokenTTL),
	); err != nil {
		return
	}
	link := fmt.Sprintf("%s/verify-email?token=%s", a.appURL(), raw)
	subject := "Confirmá tu correo en Tico Pay"
	body := fmt.Sprintf(
		`<div style="font-family:system-ui,sans-serif;max-width:480px">
		  <h2>¡Bienvenido a Tico Pay, %s! 🇨🇷</h2>
		  <p>Confirmá tu correo para asegurar tu cuenta (el enlace vence en 24 horas):</p>
		  <p><a href="%s" style="background:#002b7f;color:#fff;padding:12px 20px;border-radius:8px;text-decoration:none;display:inline-block">Confirmar mi correo</a></p>
		  <p style="color:#64748b;font-size:13px">Si no creaste esta cuenta, ignorá este correo.</p>
		</div>`,
		html.EscapeString(name), link)
	_ = a.mailer.Send(ctx, to, subject, body)
}

func (a *App) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		writeError(w, http.StatusBadRequest, "enlace inválido o expirado")
		return
	}
	ctx := r.Context()

	tx, err := a.pool.Begin(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer tx.Rollback(ctx)

	// Atomically claim the token (single-use even under concurrent requests).
	var uid string
	err = tx.QueryRow(ctx,
		`UPDATE email_verification_tokens SET used_at = now()
		 WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
		 RETURNING user_id`,
		hashToken(token),
	).Scan(&uid)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusBadRequest, "enlace inválido o expirado")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	if _, err := tx.Exec(ctx, `UPDATE users SET email_verified = true WHERE id = $1`, uid); err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo completar la operación")
		return
	}
	// Drop any other outstanding verification links for this user.
	if _, err := tx.Exec(ctx,
		`UPDATE email_verification_tokens SET used_at = now() WHERE user_id = $1 AND used_at IS NULL`, uid,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo completar la operación")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo completar la operación")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleResendVerification re-sends the verification link to the logged-in
// user (no-op if already verified).
func (a *App) handleResendVerification(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := userID(r)

	var email, name string
	var verified bool
	if err := a.pool.QueryRow(ctx,
		`SELECT email, full_name, COALESCE(email_verified,false) FROM users WHERE id = $1`, uid,
	).Scan(&email, &name, &verified); err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo cargar el usuario")
		return
	}
	if !verified {
		// Drop prior unused tokens so only the freshest link works.
		_, _ = a.pool.Exec(ctx,
			`UPDATE email_verification_tokens SET used_at = now() WHERE user_id = $1 AND used_at IS NULL`, uid)
		go func() {
			bg, cancel := bgEmailCtx()
			defer cancel()
			a.sendVerificationEmail(bg, uid, email, name)
		}()
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
