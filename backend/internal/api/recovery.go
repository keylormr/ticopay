package api

import (
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"ticopay/backend/internal/auth"
)

// Recovery codes let a passwordless user back into their account when every
// passkey is lost. We mint a fresh batch (replacing any prior unused codes),
// hash each with bcrypt, and show the plaintext exactly once.

const (
	recoveryCodeCount = 10
	// 32-char alphabet without ambiguous glyphs (no 0/O/1/I/L) so codes are
	// easy to read off a screen and type back.
	recoveryAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	recoveryCodeLen  = 8 // chars of entropy; 32^8 ≈ 1.1e12 combinations
)

// genRecoveryCode returns a code like "ABCD-EF23" (dash is cosmetic).
func genRecoveryCode() (string, error) {
	buf := make([]byte, recoveryCodeLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, recoveryCodeLen)
	for i, b := range buf {
		out[i] = recoveryAlphabet[int(b)%len(recoveryAlphabet)]
	}
	return string(out[:4]) + "-" + string(out[4:]), nil
}

// normalizeRecoveryCode strips formatting so "abcd-ef23", "ABCD EF23" and
// "ABCDEF23" all hash to the same canonical value.
func normalizeRecoveryCode(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.NewReplacer("-", "", " ", "").Replace(s)
	return s
}

// handleGenerateRecoveryCodes mints a fresh batch for the logged-in user,
// invalidating any previous unused codes, and returns the plaintext once.
func (a *App) handleGenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := userID(r)

	codes := make([]string, 0, recoveryCodeCount)
	hashes := make([]string, 0, recoveryCodeCount)
	for i := 0; i < recoveryCodeCount; i++ {
		code, err := genRecoveryCode()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "no se pudieron generar los códigos")
			return
		}
		hash, err := auth.HashPassword(normalizeRecoveryCode(code))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "no se pudieron generar los códigos")
			return
		}
		codes = append(codes, code)
		hashes = append(hashes, hash)
	}

	tx, err := a.pool.Begin(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer tx.Rollback(ctx)

	// Drop the old batch entirely: regenerating replaces, never accumulates.
	if _, err := tx.Exec(ctx, `DELETE FROM passkey_recovery_codes WHERE user_id = $1`, uid); err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudieron generar los códigos")
		return
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO passkey_recovery_codes (user_id, code_hash)
		 SELECT $1, h FROM unnest($2::text[]) AS h`,
		uid, hashes,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudieron generar los códigos")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudieron generar los códigos")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"codes": codes})
}

// handleRecoveryStatus reports how many unused codes remain (we can't show the
// codes themselves — only their hashes are stored).
func (a *App) handleRecoveryStatus(w http.ResponseWriter, r *http.Request) {
	var remaining int
	if err := a.pool.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM passkey_recovery_codes WHERE user_id = $1 AND used_at IS NULL`,
		userID(r),
	).Scan(&remaining); err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudieron cargar los códigos")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"remaining": remaining})
}

// handleRecoveryLogin signs a user in with a one-time recovery code, consuming
// it on success. Shares the brute-force guard with password login.
func (a *App) handleRecoveryLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	code := normalizeRecoveryCode(req.Code)
	if email == "" || code == "" {
		writeError(w, http.StatusBadRequest, "código de recuperación inválido")
		return
	}

	// Reuse the login lockout, keyed per email, so codes can't be brute-forced.
	guardKey := "recovery:" + email
	if loginAttempts.locked(guardKey) {
		writeError(w, http.StatusTooManyRequests, "demasiados intentos, probá de nuevo en unos minutos")
		return
	}

	ctx := r.Context()
	var uid string
	if err := a.pool.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, email).Scan(&uid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			loginAttempts.fail(guardKey)
			writeError(w, http.StatusUnauthorized, "código de recuperación inválido")
			return
		}
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	matchedID, err := a.consumeRecoveryCode(ctx, uid, code)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo completar la operación")
		return
	}
	if matchedID == "" {
		loginAttempts.fail(guardKey)
		writeError(w, http.StatusUnauthorized, "código de recuperación inválido")
		return
	}

	u, err := a.fetchUser(ctx, uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo cargar el usuario")
		return
	}
	accounts, err := a.fetchAccounts(ctx, uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo cargar las cuentas")
		return
	}
	loginAttempts.reset(guardKey)
	a.issueAuthResponse(w, http.StatusOK, u, accounts)
}

// consumeRecoveryCode finds the unused code matching the plaintext, marks it
// used, and returns its id. Returns "" (no error) when nothing matches.
func (a *App) consumeRecoveryCode(ctx context.Context, uid, code string) (string, error) {
	rows, err := a.pool.Query(ctx,
		`SELECT id, code_hash FROM passkey_recovery_codes WHERE user_id = $1 AND used_at IS NULL`, uid)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	type entry struct{ id, hash string }
	var entries []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.id, &e.hash); err != nil {
			return "", err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	for _, e := range entries {
		if auth.CheckPassword(e.hash, code) {
			if _, err := a.pool.Exec(ctx,
				`UPDATE passkey_recovery_codes SET used_at = now() WHERE id = $1 AND used_at IS NULL`, e.id,
			); err != nil {
				return "", err
			}
			return e.id, nil
		}
	}
	return "", nil
}
