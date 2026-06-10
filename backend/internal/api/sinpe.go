package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// sinpeComprobante derives a 12-digit numeric reference (SINPE-style) from a
// transaction id, so the confirmation looks like a real receipt.
func sinpeComprobante(txID string) string {
	u, err := uuid.Parse(txID)
	if err != nil {
		return txID
	}
	var n uint64
	for i := 0; i < 6; i++ {
		n = n<<8 | uint64(u[i])
	}
	return fmt.Sprintf("%012d", n%1_000_000_000_000)
}

// handleSinpe sends a SINPE Móvil transfer: by phone number, in colones,
// instant. (Demo: settles on Tico Pay's internal ledger.)
func (a *App) handleSinpe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ToPhone     string  `json:"toPhone"`
		Amount      float64 `json:"amount"`
		Description string  `json:"description"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}

	phone := normalizePhone(req.ToPhone)
	if len(phone) != 8 {
		writeError(w, http.StatusBadRequest, "ingresá un número de teléfono de 8 dígitos")
		return
	}
	amountCents := toMinor(req.Amount, "CRC")
	if amountCents <= 0 {
		writeError(w, http.StatusBadRequest, "el monto debe ser mayor a cero")
		return
	}

	desc := strings.TrimSpace(req.Description)
	if desc == "" {
		desc = "SINPE Móvil"
	}

	txID, newBalance, err := a.transfer(r.Context(), userID(r), phone, "CRC", amountCents, desc, "sinpe")
	if err != nil {
		writeTransferError(w, err)
		return
	}

	// Recipient name for the receipt (best-effort).
	_, name, _ := a.resolveUserID(r.Context(), phone)

	writeJSON(w, http.StatusCreated, map[string]any{
		"comprobante":   sinpeComprobante(txID),
		"recipientName": name,
		"amountCents":   amountCents,
		"currency":      "CRC",
		"newBalance":    newBalance,
		"at":            time.Now().UTC().Format(time.RFC3339),
	})
}
