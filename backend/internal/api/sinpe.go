package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
// instant. (Demo: settles on TuanisPay's internal ledger.)
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

	// Recipient name for the receipt (best-effort), resolved before the money
	// transaction so we don't hold a second pooled connection inside it.
	_, recipientName, _ := a.resolveUserID(r.Context(), phone)

	fp := "sinpe|" + phone + "|" + strconv.FormatInt(amountCents, 10)
	a.idempotent(w, r, idempotencyKey(r), fp, func(tx pgx.Tx) (int, map[string]any, error) {
		txID, newBalance, err := a.transferTx(r.Context(), tx, userID(r), phone, "CRC", amountCents, desc, "sinpe")
		if err != nil {
			return 0, nil, err
		}
		return http.StatusCreated, map[string]any{
			"comprobante":   sinpeComprobante(txID),
			"recipientName": recipientName,
			"amountCents":   amountCents,
			"currency":      "CRC",
			"newBalance":    newBalance,
			"simulated":     true,
			"at":            time.Now().UTC().Format(time.RFC3339),
		}, nil
	})
}
