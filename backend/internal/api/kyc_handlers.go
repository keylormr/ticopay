package api

import (
	"net/http"
	"strings"
)

// validateCRID checks Costa Rican identification number formats.
//   - fisica   (cédula nacional): 9 digits, province 1-7
//   - juridica (cédula jurídica): 10 digits, starts with 3
//   - dimex    (extranjero):      11 or 12 digits
func validateCRID(idType, raw string) (normalized string, ok bool) {
	digits := nonDigits.ReplaceAllString(raw, "")
	switch idType {
	case "fisica":
		if len(digits) == 9 && digits[0] >= '1' && digits[0] <= '7' {
			return digits, true
		}
	case "juridica":
		if len(digits) == 10 && digits[0] == '3' {
			return digits, true
		}
	case "dimex":
		if len(digits) == 11 || len(digits) == 12 {
			return digits, true
		}
	}
	return "", false
}

func (a *App) handleSubmitKYC(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDType   string `json:"idType"`
		IDNumber string `json:"idNumber"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.IDType = strings.ToLower(strings.TrimSpace(req.IDType))

	normalized, ok := validateCRID(req.IDType, req.IDNumber)
	if !ok {
		writeError(w, http.StatusBadRequest, "número de identificación inválido para el tipo seleccionado")
		return
	}

	// Demo environment auto-verifies; a real deployment would queue manual or
	// third-party verification (e.g. TSE / Registro Nacional).
	if _, err := a.pool.Exec(r.Context(),
		`UPDATE users SET id_type = $1, id_number = $2, kyc_status = 'verified' WHERE id = $3`,
		req.IDType, normalized, userID(r),
	); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save verification")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"kycStatus": "verified",
		"idType":    req.IDType,
		"idNumber":  normalized,
	})
}
