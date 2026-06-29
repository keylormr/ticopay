package api

import "testing"

func TestValidateStaffPassword(t *testing.T) {
	cases := []struct {
		name string
		pw   string
		ok   bool
	}{
		{"too short", "Abc1", false},
		{"nine chars mixed", "Abcdefg12", false}, // 9 < 10
		{"ten chars letters only", "abcdefghij", false},
		{"ten chars digits only", "0123456789", false},
		{"ten chars mixed ok", "abcdefgh12", true},
		{"long mixed ok", "Contrasena-Segura-2026", true},
		{"empty", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			msg := validateStaffPassword(c.pw)
			if c.ok && msg != "" {
				t.Fatalf("validateStaffPassword(%q) rejected: %q", c.pw, msg)
			}
			if !c.ok && msg == "" {
				t.Fatalf("validateStaffPassword(%q) accepted, want rejection", c.pw)
			}
		})
	}
}
