package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

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

// idempotent runs fn at most once per key and replays the stored JSON response
// verbatim on retries, so a network re-send never moves money twice. The
// Idempotency-Key header is REQUIRED for the money POSTs that use this helper
// (a request without one is rejected, never run undeduped). fn returns
// (status, body, error); on error the key is released so the client can retry.
func (a *App) idempotent(w http.ResponseWriter, r *http.Request, key string, fn func() (int, map[string]any, error)) {
	if key == "" {
		writeError(w, http.StatusBadRequest, "falta la cabecera Idempotency-Key")
		return
	}

	ctx := r.Context()
	ct, err := a.pool.Exec(ctx,
		`INSERT INTO idempotency_keys (key, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		key, userID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Key already claimed by an earlier request: replay its stored response.
	if ct.RowsAffected() == 0 {
		var status *int
		var resp []byte
		if err := a.pool.QueryRow(ctx,
			`SELECT status_code, response FROM idempotency_keys WHERE key = $1`, key,
		).Scan(&status, &resp); err != nil {
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		if status == nil || resp == nil {
			// The first request is still in flight; ask the client to wait.
			writeError(w, http.StatusConflict, "operación en curso, esperá un momento")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(*status)
		_, _ = w.Write(resp)
		return
	}

	// We own the key: do the work once.
	status, payload, err := fn()

	// Record the outcome with a context decoupled from the client request. fn
	// may have already committed the money move; if the client disconnected,
	// r.Context() is cancelled but the result MUST still be persisted, or the
	// key would be stranded (NULL status) and every retry would 409 forever.
	// The money still moved exactly once regardless of this write.
	wctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err != nil {
		// fn failed and rolled back its own transaction: release the key so the
		// client can retry the operation.
		if _, derr := a.pool.Exec(wctx, `DELETE FROM idempotency_keys WHERE key = $1`, key); derr != nil {
			Logger.Error("idempotency key release failed", "key", key, "error", derr)
		}
		writeTransferError(w, err)
		return
	}
	raw, _ := json.Marshal(payload)
	if _, uerr := a.pool.Exec(wctx,
		`UPDATE idempotency_keys SET status_code = $1, response = $2 WHERE key = $3`, status, raw, key); uerr != nil {
		Logger.Error("idempotency key finalize failed", "key", key, "error", uerr)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}
