package api

import "unicode"

// minStaffPasswordLen is the floor for back-office accounts, stricter than the
// 8-char minimum self-registered customers get: staff hold privileged access,
// so their initial password must be harder to guess.
const minStaffPasswordLen = 10

// validateStaffPassword enforces the staff password policy: at least
// minStaffPasswordLen characters mixing letters and digits. Returns "" when
// acceptable, otherwise a (Spanish, localizable) error message.
func validateStaffPassword(pw string) string {
	if len([]rune(pw)) < minStaffPasswordLen {
		return "la contraseña del personal debe tener al menos 10 caracteres"
	}
	var hasLetter, hasDigit bool
	for _, r := range pw {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return "la contraseña debe combinar letras y números"
	}
	return ""
}
