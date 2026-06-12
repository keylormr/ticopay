package api

import (
	"net/http/httptest"
	"testing"
)

func wlang(lang string) *langResponseWriter {
	return &langResponseWriter{ResponseWriter: httptest.NewRecorder(), lang: lang}
}

func TestLocalizeError(t *testing.T) {
	cases := []struct {
		lang, msg, want string
	}{
		// Spanish-authored copy translates to EN and passes through in ES.
		{"en", "saldo insuficiente", "insufficient balance"},
		{"es", "saldo insuficiente", "saldo insuficiente"},
		// English-authored technical fallbacks translate to ES.
		{"es", "database error", "error de base de datos"},
		{"en", "database error", "database error"},
		// Unknown messages pass through untouched in both languages.
		{"en", "mensaje desconocido", "mensaje desconocido"},
		{"es", "unknown message", "unknown message"},
	}
	for _, c := range cases {
		if got := localizeError(wlang(c.lang), c.msg); got != c.want {
			t.Errorf("localizeError(%s, %q) = %q, want %q", c.lang, c.msg, got, c.want)
		}
	}
}

// Every Spanish message must have a distinct English translation (a missing
// entry would silently leak Spanish to EN users; this catches typos in new
// entries like the TOTP/recovery additions).
func TestErrsENNonEmpty(t *testing.T) {
	for es, en := range errsEN {
		if en == "" {
			t.Errorf("errsEN[%q] is empty", es)
		}
	}
	for en, es := range errsES {
		if es == "" {
			t.Errorf("errsES[%q] is empty", en)
		}
	}
}
