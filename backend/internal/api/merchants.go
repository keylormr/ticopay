package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"ticopay/backend/internal/models"
)

// handleCreateMerchant registers a business for the current user. It starts
// `pending` and must be approved by an admin before it can charge.
func (a *App) handleCreateMerchant(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		Category  string `json:"category"`
		LegalName string `json:"legalName"`
		IDType    string `json:"idType"`
		IDNumber  string `json:"idNumber"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "el nombre del comercio es obligatorio")
		return
	}
	// Light KYC: if an id is supplied, validate its Costa Rican format.
	var idType, idNumber *string
	req.IDType = strings.ToLower(strings.TrimSpace(req.IDType))
	if req.IDType != "" || strings.TrimSpace(req.IDNumber) != "" {
		normalized, ok := validateCRID(req.IDType, req.IDNumber)
		if !ok {
			writeError(w, http.StatusBadRequest, "número de identificación inválido para el tipo seleccionado")
			return
		}
		idType, idNumber = &req.IDType, &normalized
	}

	var m models.Merchant
	if err := a.pool.QueryRow(r.Context(),
		`INSERT INTO merchants (owner_id, name, category, legal_name, id_type, id_number)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, name, category, legal_name, status, reject_reason, commission_bps, created_at`,
		userID(r), req.Name, strings.TrimSpace(req.Category), strings.TrimSpace(req.LegalName), idType, idNumber,
	).Scan(&m.ID, &m.Name, &m.Category, &m.LegalName, &m.Status, &m.RejectReason, &m.CommissionBps, &m.CreatedAt); err != nil {
		writeError(w, http.StatusInternalServerError, "could not create merchant")
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

// handleListMerchants returns the current user's merchants.
func (a *App) handleListMerchants(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(),
		`SELECT id, name, category, legal_name, status, reject_reason, commission_bps, created_at
		 FROM merchants WHERE owner_id = $1 ORDER BY created_at DESC`, userID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load merchants")
		return
	}
	defer rows.Close()

	list := make([]models.Merchant, 0)
	for rows.Next() {
		var m models.Merchant
		if err := rows.Scan(&m.ID, &m.Name, &m.Category, &m.LegalName, &m.Status,
			&m.RejectReason, &m.CommissionBps, &m.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "could not read merchants")
			return
		}
		list = append(list, m)
	}
	writeJSON(w, http.StatusOK, map[string]any{"merchants": list})
}

// handleCreateMerchantCharge emits a QR charge (a payment_request tied to the
// merchant). Only a verified merchant owned by the caller can charge. Amount may
// be fixed (>0) or open (the payer chooses).
func (a *App) handleCreateMerchantCharge(w http.ResponseWriter, r *http.Request) {
	mid := chi.URLParam(r, "id")
	var req struct {
		Amount      float64 `json:"amount"`
		Currency    string  `json:"currency"`
		Description string  `json:"description"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
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

	// The merchant must belong to the caller and be verified.
	var status string
	err := a.pool.QueryRow(r.Context(),
		`SELECT status FROM merchants WHERE id = $1 AND owner_id = $2`, mid, userID(r),
	).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "comercio no encontrado")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load merchant")
		return
	}
	if status != "verified" {
		writeError(w, http.StatusForbidden, "el comercio aún no está verificado")
		return
	}

	var amountCents *int64
	if req.Amount > 0 {
		c := toMinor(req.Amount, currency)
		amountCents = &c
	}

	var id string
	if err := a.pool.QueryRow(r.Context(),
		`INSERT INTO payment_requests (requester_id, merchant_id, amount_cents, currency, description)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		userID(r), mid, amountCents, currency, strings.TrimSpace(req.Description),
	).Scan(&id); err != nil {
		writeError(w, http.StatusInternalServerError, "could not create charge")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "currency": currency})
}
