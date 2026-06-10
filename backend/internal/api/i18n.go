package api

import (
	"net/http"
	"strings"
)

// langResponseWriter carries the request language so writeError can translate
// without changing every call site.
type langResponseWriter struct {
	http.ResponseWriter
	lang string
}

func langOf(w http.ResponseWriter) string {
	if lw, ok := w.(*langResponseWriter); ok {
		return lw.lang
	}
	return "es"
}

// withLang reads the client language (X-Lang header, else Accept-Language) and
// wraps the writer. Must be the innermost middleware so handlers receive it.
func withLang(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := "es"
		if strings.EqualFold(r.Header.Get("X-Lang"), "en") ||
			strings.HasPrefix(strings.ToLower(r.Header.Get("Accept-Language")), "en") {
			lang = "en"
		}
		next.ServeHTTP(&langResponseWriter{ResponseWriter: w, lang: lang}, r)
	})
}

// errsEN maps the Spanish (default) error messages to English.
var errsEN = map[string]string{
	"solicitud inválida":             "invalid request",
	"moneda no soportada":            "unsupported currency",
	"el monto debe ser mayor a cero": "amount must be greater than zero",

	"saldo insuficiente":                                  "insufficient balance",
	"ingresá un número de teléfono de 8 dígitos":          "enter an 8-digit phone number",
	"destinatario no encontrado":                          "recipient not found",
	"no podés enviarte dinero a vos mismo":                "you can't send money to yourself",
	"no se pudo completar la operación":                   "the operation couldn't be completed",
	"el destinatario (correo o teléfono) es obligatorio":  "a recipient (email or phone) is required",
	"elegí dos monedas distintas":                         "pick two different currencies",
	"tipo de cambio no disponible":                        "exchange rate unavailable",
	"monto muy pequeño para convertir":                    "amount too small to convert",

	"ingresá un correo válido":                        "a valid email is required",
	"la contraseña debe tener al menos 8 caracteres":  "password must be at least 8 characters",
	"el nombre completo es obligatorio":               "full name is required",
	"ese correo o teléfono ya está registrado":        "that email or phone is already registered",
	"correo o contraseña incorrectos":                 "invalid email or password",
	"demasiados intentos, probá de nuevo en unos minutos": "too many attempts, try again in a few minutes",

	"no encontramos a esa persona en Tico Pay":  "we couldn't find that person on Tico Pay",
	"cobro no encontrado":                        "request not found",
	"este cobro ya fue pagado o cancelado":       "this request was already paid or cancelled",

	"el nombre de la vaquita es obligatorio": "the pool name is required",
	"vaquita no encontrada":                  "pool not found",
	"esta vaquita está cerrada":              "this pool is closed",

	"número de identificación inválido para el tipo seleccionado": "invalid ID number for the selected type",

	"servicio no válido":           "invalid service",
	"la referencia es obligatoria": "the reference is required",

	"passkeys no disponibles":               "passkeys unavailable",
	"esta cuenta no tiene llave de acceso":  "this account has no passkey",
	"no se pudo verificar la llave de acceso": "couldn't verify the passkey",
	"no se pudo registrar la llave":          "couldn't register the passkey",
	"no encontramos esa cuenta":            "we couldn't find that account",
	"sesión inválida o expirada":           "session invalid or expired",
	"sesión inválida":                      "invalid session",
	"id inválido":                          "invalid id",
	"credencial inválida":                  "invalid credential",

	"no se pudieron generar los códigos": "couldn't generate the codes",
	"no se pudieron cargar los códigos":  "couldn't load the codes",
	"código de recuperación inválido":    "invalid recovery code",
}

// localizeError returns the message in the writer's language.
func localizeError(w http.ResponseWriter, msg string) string {
	if langOf(w) == "en" {
		if en, ok := errsEN[msg]; ok {
			return en
		}
	}
	return msg
}
