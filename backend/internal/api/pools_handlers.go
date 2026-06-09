package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"ticopay/backend/internal/models"
)

func (a *App) handleCreatePool(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		GoalAmount  float64 `json:"goalAmount"`
		Currency    string  `json:"currency"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "el nombre de la vaquita es obligatorio")
		return
	}
	currency := req.Currency
	if currency == "" {
		currency = "CRC"
	}
	if !validCurrency(currency) {
		writeError(w, http.StatusBadRequest, "moneda no soportada")
		return
	}
	goalCents := toMinor(req.GoalAmount, currency)
	if goalCents < 0 {
		goalCents = 0
	}

	var id string
	if err := a.pool.QueryRow(r.Context(),
		`INSERT INTO pools (owner_id, name, description, goal_cents, currency)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		userID(r), req.Name, strings.TrimSpace(req.Description), goalCents, currency,
	).Scan(&id); err != nil {
		writeError(w, http.StatusInternalServerError, "could not create pool")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (a *App) queryPools(ctx context.Context, sql, uid string) ([]models.Pool, error) {
	rows, err := a.pool.Query(ctx, sql, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]models.Pool, 0)
	for rows.Next() {
		var p models.Pool
		if err := rows.Scan(&p.ID, &p.OwnerName, &p.IsOwner, &p.Name, &p.Description,
			&p.GoalCents, &p.RaisedCents, &p.Currency, &p.Status, &p.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (a *App) handleListPools(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := userID(r)

	mine, err := a.queryPools(ctx, `
		SELECT p.id, ou.full_name, (p.owner_id = $1), p.name, p.description, p.goal_cents,
		       COALESCE((SELECT SUM(amount_cents) FROM pool_contributions c WHERE c.pool_id = p.id), 0)::bigint,
		       p.currency, p.status, p.created_at
		FROM pools p JOIN users ou ON ou.id = p.owner_id
		WHERE p.owner_id = $1
		ORDER BY p.created_at DESC LIMIT 50`, uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load pools")
		return
	}

	joined, err := a.queryPools(ctx, `
		SELECT p.id, ou.full_name, (p.owner_id = $1), p.name, p.description, p.goal_cents,
		       COALESCE((SELECT SUM(amount_cents) FROM pool_contributions c WHERE c.pool_id = p.id), 0)::bigint,
		       p.currency, p.status, p.created_at
		FROM pools p JOIN users ou ON ou.id = p.owner_id
		WHERE p.owner_id <> $1
		  AND EXISTS (SELECT 1 FROM pool_contributions c WHERE c.pool_id = p.id AND c.user_id = $1)
		ORDER BY p.created_at DESC LIMIT 50`, uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load pools")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"mine": mine, "joined": joined})
}

func (a *App) handleGetPool(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	var p models.Pool
	err := a.pool.QueryRow(ctx, `
		SELECT p.id, ou.full_name, (p.owner_id = $2), p.name, p.description, p.goal_cents,
		       COALESCE((SELECT SUM(amount_cents) FROM pool_contributions c WHERE c.pool_id = p.id), 0)::bigint,
		       p.currency, p.status, p.created_at
		FROM pools p JOIN users ou ON ou.id = p.owner_id
		WHERE p.id = $1`, id, userID(r),
	).Scan(&p.ID, &p.OwnerName, &p.IsOwner, &p.Name, &p.Description,
		&p.GoalCents, &p.RaisedCents, &p.Currency, &p.Status, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "vaquita no encontrada")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load pool")
		return
	}

	rows, err := a.pool.Query(ctx, `
		SELECT u.full_name, c.amount_cents, c.created_at
		FROM pool_contributions c JOIN users u ON u.id = c.user_id
		WHERE c.pool_id = $1 ORDER BY c.created_at DESC LIMIT 100`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load contributions")
		return
	}
	defer rows.Close()

	contribs := make([]models.PoolContribution, 0)
	for rows.Next() {
		var c models.PoolContribution
		if err := rows.Scan(&c.Name, &c.AmountCents, &c.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "could not read contributions")
			return
		}
		contribs = append(contribs, c)
	}

	writeJSON(w, http.StatusOK, map[string]any{"pool": p, "contributions": contribs})
}

func (a *App) handleContributePool(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Amount float64 `json:"amount"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "el monto debe ser mayor a cero")
		return
	}

	ctx := r.Context()
	var (
		ownerID  string
		currency string
		status   string
		name     string
	)
	err := a.pool.QueryRow(ctx,
		`SELECT owner_id, currency, status, name FROM pools WHERE id = $1`, id,
	).Scan(&ownerID, &currency, &status, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "vaquita no encontrada")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load pool")
		return
	}
	if status != "open" {
		writeError(w, http.StatusConflict, "esta vaquita está cerrada")
		return
	}

	amountCents := toMinor(req.Amount, currency)
	txID, err := a.transferToUser(ctx, userID(r), ownerID, currency, amountCents, "Aporte a vaquita: "+name, "pool")
	if err != nil {
		writeTransferError(w, err)
		return
	}

	if _, err := a.pool.Exec(ctx,
		`INSERT INTO pool_contributions (pool_id, user_id, amount_cents, tx_id) VALUES ($1, $2, $3, $4)`,
		id, userID(r), amountCents, txID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "aporte enviado pero no registrado")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"status": "ok", "amountCents": amountCents, "currency": currency})
}
