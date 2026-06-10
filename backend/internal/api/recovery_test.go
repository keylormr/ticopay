package api

import (
	"strings"
	"testing"

	"ticopay/backend/internal/auth"
)

func TestGenRecoveryCodeFormat(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		code, err := genRecoveryCode()
		if err != nil {
			t.Fatalf("genRecoveryCode: %v", err)
		}
		// Shape: XXXX-XXXX from the unambiguous alphabet.
		if len(code) != recoveryCodeLen+1 || code[4] != '-' {
			t.Fatalf("unexpected format: %q", code)
		}
		for _, c := range code {
			if c == '-' {
				continue
			}
			if !strings.ContainsRune(recoveryAlphabet, c) {
				t.Fatalf("code %q has char outside alphabet: %q", code, c)
			}
		}
		seen[code] = true
	}
	// Collisions in 200 draws from 32^8 should be effectively impossible.
	if len(seen) < 200 {
		t.Fatalf("expected 200 unique codes, got %d", len(seen))
	}
}

func TestNormalizeRecoveryCode(t *testing.T) {
	want := "ABCDEF23"
	for _, in := range []string{"ABCD-EF23", "abcd-ef23", "  ABCD EF23 ", "abcdef23", "AbCd-Ef23"} {
		if got := normalizeRecoveryCode(in); got != want {
			t.Errorf("normalizeRecoveryCode(%q) = %q, want %q", in, got, want)
		}
	}
}

// A code hashed in its canonical form must verify regardless of how the user
// types it back (dashes, spaces, casing) — this is what the login path relies on.
func TestRecoveryCodeRoundTrip(t *testing.T) {
	code, err := genRecoveryCode()
	if err != nil {
		t.Fatalf("genRecoveryCode: %v", err)
	}
	hash, err := auth.HashPassword(normalizeRecoveryCode(code))
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	for _, variant := range []string{code, strings.ToLower(code), strings.ReplaceAll(code, "-", ""), strings.ReplaceAll(code, "-", " ")} {
		if !auth.CheckPassword(hash, normalizeRecoveryCode(variant)) {
			t.Errorf("variant %q failed to verify", variant)
		}
	}
	// A different code must not verify.
	other, _ := genRecoveryCode()
	if auth.CheckPassword(hash, normalizeRecoveryCode(other)) {
		t.Errorf("unrelated code %q verified against hash", other)
	}
}
