package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/pquerna/otp/totp"
)

// TOTP 2FA (authenticator apps) as an alternative to passkeys. Setup stores
// an unconfirmed secret; only after the user proves a valid code does it
// start gating password login (handleLogin returns 428 until a code comes).

// totpEnabled reports whether the user has a confirmed TOTP secret.
func (a *App) totpEnabled(ctx context.Context, uid string) (bool, string, error) {
	var secret string
	err := a.pool.QueryRow(ctx,
		`SELECT secret FROM user_totp WHERE user_id = $1 AND confirmed`, uid).Scan(&secret)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return true, secret, nil
}

func normalizeTotpCode(s string) string {
	return strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(s))
}

// handleTotpStatus reports whether 2FA is active for the logged-in user.
func (a *App) handleTotpStatus(w http.ResponseWriter, r *http.Request) {
	enabled, _, err := a.totpEnabled(r.Context(), userID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": enabled})
}

// handleTotpSetup mints a fresh secret (unconfirmed) and returns the otpauth
// URL for the authenticator app plus the base32 secret for manual entry.
func (a *App) handleTotpSetup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := userID(r)

	var email string
	if err := a.pool.QueryRow(ctx, `SELECT email FROM users WHERE id = $1`, uid).Scan(&email); err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo cargar el usuario")
		return
	}

	key, err := totp.Generate(totp.GenerateOpts{Issuer: "Tico Pay", AccountName: email})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo configurar el 2FA")
		return
	}

	// Setup always overwrites and resets confirmed: an abandoned or repeated
	// setup never locks the account, because login only gates on confirmed.
	if _, err := a.pool.Exec(ctx,
		`INSERT INTO user_totp (user_id, secret, confirmed) VALUES ($1, $2, false)
		 ON CONFLICT (user_id) DO UPDATE SET secret = $2, confirmed = false, created_at = now()`,
		uid, key.Secret(),
	); err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo configurar el 2FA")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"secret":     key.Secret(),
		"otpauthUrl": key.URL(),
	})
}

// handleTotpConfirm validates the first code from the user's app and turns
// the secret on. From here password login requires a code.
func (a *App) handleTotpConfirm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	ctx := r.Context()
	uid := userID(r)

	var secret string
	err := a.pool.QueryRow(ctx, `SELECT secret FROM user_totp WHERE user_id = $1`, uid).Scan(&secret)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusBadRequest, "primero configurá el 2FA")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if !totp.Validate(normalizeTotpCode(req.Code), secret) {
		writeError(w, http.StatusUnauthorized, "código 2FA inválido")
		return
	}
	if _, err := a.pool.Exec(ctx,
		`UPDATE user_totp SET confirmed = true WHERE user_id = $1`, uid); err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleTotpDisable turns 2FA off; requires a currently-valid code so a
// hijacked session can't silently weaken the account.
func (a *App) handleTotpDisable(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	ctx := r.Context()
	uid := userID(r)

	enabled, secret, err := a.totpEnabled(ctx, uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if !enabled {
		// Nothing confirmed; clear any half-finished setup and report ok.
		_, _ = a.pool.Exec(ctx, `DELETE FROM user_totp WHERE user_id = $1`, uid)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if !totp.Validate(normalizeTotpCode(req.Code), secret) {
		writeError(w, http.StatusUnauthorized, "código 2FA inválido")
		return
	}
	if _, err := a.pool.Exec(ctx, `DELETE FROM user_totp WHERE user_id = $1`, uid); err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
