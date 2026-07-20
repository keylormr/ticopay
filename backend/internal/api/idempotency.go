package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// errIdemInFlight signals that the idempotency key is already claimed by another
// (possibly concurrent) request, so this attempt must roll back and replay
// instead of moving money a second time.
var errIdemInFlight = errors.New("idempotency key in flight")

// idempotencyKey returns the client-supplied Idempotency-Key namespaced by the
// authenticated user, so keys can't collide across accounts. Empty when the
// header is absent (the caller then runs without dedupe).
func idempotencyKey(r *http.Request) string {
	k := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if k == "" {
		return ""
	}
	return userID(r) + ":" + k
}

// idemHash reduces a canonical request fingerprint to a fixed-size hex digest
// stored next to the key, so a key reused with a different payload is caught.
func idemHash(fingerprint string) string {
	if fingerprint == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(fingerprint))
	return hex.EncodeToString(sum[:])
}

// idempotent runs fn at most once per key and replays the stored JSON response
// verbatim on retries, so a network re-send never moves money twice.
//
// The key row and fn's money move share ONE transaction: they commit or roll
// back together. That is what closes the double-spend window a plain
// dedupe-then-move design leaves open — if the COMMIT is ambiguous (the network
// drops right as Postgres durably commits), the money moved but the caller sees
// an error; because we never release the key by hand, a retry replays the stored
// response instead of moving the money again. If the commit did NOT apply,
// neither the key nor the money persisted, so the retry re-runs cleanly.
//
// fn does its work on the provided tx and must NOT open its own transaction. The
// Idempotency-Key header is REQUIRED (a request without one is rejected).
// fingerprint is a canonical description of the request; reusing a key with a
// different fingerprint yields 422 rather than replaying an unrelated response.
func (a *App) idempotent(w http.ResponseWriter, r *http.Request, key, fingerprint string, fn func(pgx.Tx) (int, map[string]any, error)) {
	if key == "" {
		writeError(w, http.StatusBadRequest, "falta la cabecera Idempotency-Key")
		return
	}
	ctx := r.Context()
	hash := idemHash(fingerprint)

	// Fast path: a finished key replays without opening a work transaction; a
	// fingerprint mismatch returns 422. A missing or still-in-flight key falls
	// through to the claim below.
	if a.tryReplay(ctx, w, key, hash) {
		return
	}

	// First writer: claim the key and do the money move in the SAME transaction.
	var outStatus int
	var outRaw []byte
	txErr := a.inTx(ctx, func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx,
			`INSERT INTO idempotency_keys (key, user_id, request_hash) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			key, userID(r), nullIfEmpty(hash))
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			// The row already exists (an earlier request, or one racing us that
			// committed first). Abort and let the caller replay it.
			return errIdemInFlight
		}
		status, payload, ferr := fn(tx)
		if ferr != nil {
			return ferr // rollback undoes the claim AND the money move together
		}
		raw, merr := json.Marshal(payload)
		if merr != nil {
			return errTransferOther
		}
		if _, err := tx.Exec(ctx,
			`UPDATE idempotency_keys SET status_code = $1, response = $2 WHERE key = $3`, status, raw, key); err != nil {
			return err
		}
		outStatus, outRaw = status, raw
		return nil
	})

	if txErr != nil {
		if errors.Is(txErr, errIdemInFlight) {
			// The key is owned by another request: replay its result, or report
			// that it's still in flight. The money move never re-runs here.
			if a.tryReplay(ctx, w, key, hash) {
				return
			}
			writeError(w, http.StatusConflict, "operación en curso, esperá un momento")
			return
		}
		// A deterministic failure (bad amount, insufficient balance, …) rolled
		// back and released the key, so the client may retry; an ambiguous commit
		// left the key with its response, so a retry replays. We never delete the
		// key by hand, which is what keeps both cases correct.
		writeTransferError(w, txErr)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(outStatus)
	_, _ = w.Write(outRaw)
}

// tryReplay writes the stored response for a finished key and returns true; it
// also enforces the fingerprint (422 on mismatch, also returning true). It
// returns false when the key does not exist yet, or exists but has no stored
// response (claimed and still in flight), leaving the caller to proceed or wait.
func (a *App) tryReplay(ctx context.Context, w http.ResponseWriter, key, hash string) bool {
	var status *int
	var resp []byte
	var storedHash *string
	err := a.pool.QueryRow(ctx,
		`SELECT status_code, response, request_hash FROM idempotency_keys WHERE key = $1`, key,
	).Scan(&status, &resp, &storedHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return true
	}
	if hash != "" && storedHash != nil && *storedHash != hash {
		writeError(w, http.StatusUnprocessableEntity, "esta clave de idempotencia ya se usó para otra operación")
		return true
	}
	if status == nil || resp == nil {
		return false // claimed but not finished yet
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(*status)
	_, _ = w.Write(resp)
	return true
}

// nullIfEmpty maps an empty fingerprint hash to a SQL NULL so keys created
// without a fingerprint don't store an empty string.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ReapIdempotencyKeys deletes idempotency keys older than the retention window
// on a fixed interval, keeping the table bounded — a key only matters within a
// client's retry window, well under the window here. It runs until ctx is
// cancelled and is safe to launch as a goroutine at startup.
func (a *App) ReapIdempotencyKeys(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			ct, err := a.pool.Exec(c, `DELETE FROM idempotency_keys WHERE created_at < now() - interval '7 days'`)
			cancel()
			if err != nil {
				Logger.Error("idempotency reap failed", "error", err)
			} else if n := ct.RowsAffected(); n > 0 {
				Logger.Info("idempotency keys reaped", "count", n)
			}
		}
	}
}
