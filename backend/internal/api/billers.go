package api

import (
	"net/http"
	"strings"
)

// Biller is a service/utility a user can pay from their wallet.
type Biller struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Category       string `json:"category"`
	Icon           string `json:"icon"`
	RefLabel       string `json:"refLabel"`
	RefPlaceholder string `json:"refPlaceholder"`
}

// Costa Rican service catalog. (Demo: payments debit the wallet and are
// recorded as 'service' transactions; a production build would settle through
// each biller's real channel.)
var billers = []Biller{
	{ID: "ice-elec", Name: "ICE — Electricidad", Category: "Electricidad", Icon: "💡", RefLabel: "Número de cuenta (NIS)", RefPlaceholder: "1234567"},
	{ID: "cnfl", Name: "CNFL", Category: "Electricidad", Icon: "🔌", RefLabel: "Número de servicio (NISE)", RefPlaceholder: "987654"},
	{ID: "aya", Name: "AyA — Acueductos", Category: "Agua", Icon: "🚰", RefLabel: "Número de servicio (NIS)", RefPlaceholder: "55667788"},
	{ID: "kolbi", Name: "Kölbi — Recarga", Category: "Telefonía", Icon: "📱", RefLabel: "Número de teléfono", RefPlaceholder: "8888-0000"},
	{ID: "cabletica", Name: "Cabletica / Tigo", Category: "Internet y cable", Icon: "📺", RefLabel: "Número de cliente", RefPlaceholder: "100200300"},
	{ID: "marchamo", Name: "Marchamo (INS)", Category: "Vehículo", Icon: "🚗", RefLabel: "Placa del vehículo", RefPlaceholder: "ABC123"},
	{ID: "rtv", Name: "Revisión técnica (RTV)", Category: "Vehículo", Icon: "🔧", RefLabel: "Placa del vehículo", RefPlaceholder: "ABC123"},
	{ID: "muni", Name: "Impuestos municipales", Category: "Municipalidad", Icon: "🏛️", RefLabel: "Número de finca", RefPlaceholder: "1-234567-890"},
	{ID: "ccss", Name: "CCSS — Cuotas", Category: "Seguridad social", Icon: "🏥", RefLabel: "Número asegurado / patronal", RefPlaceholder: "0-1234-5678"},
}

var billerByID = func() map[string]Biller {
	m := make(map[string]Biller, len(billers))
	for _, b := range billers {
		m[b.ID] = b
	}
	return m
}()

func (a *App) handleListBillers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"billers": billers})
}

func (a *App) handlePayService(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BillerID  string  `json:"billerId"`
		Reference string  `json:"reference"`
		Amount    float64 `json:"amount"`
		Currency  string  `json:"currency"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	biller, ok := billerByID[req.BillerID]
	if !ok {
		writeError(w, http.StatusBadRequest, "servicio no válido")
		return
	}
	req.Reference = strings.TrimSpace(req.Reference)
	if req.Reference == "" {
		writeError(w, http.StatusBadRequest, biller.RefLabel+" es obligatorio")
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
	amountCents := toMinor(req.Amount, currency)
	if amountCents <= 0 {
		writeError(w, http.StatusBadRequest, "el monto debe ser mayor a cero")
		return
	}

	desc := biller.Name + " · " + req.Reference
	txID, newBalance, err := a.payOut(r.Context(), userID(r), currency, amountCents, desc, "service")
	if err != nil {
		writeTransferError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": txID, "amountCents": amountCents, "currency": currency, "newBalance": newBalance,
	})
}
