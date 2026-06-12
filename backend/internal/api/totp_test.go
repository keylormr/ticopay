package api

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestNormalizeTotpCode(t *testing.T) {
	for in, want := range map[string]string{
		" 123 456 ": "123456",
		"123-456":   "123456",
		"123456":    "123456",
	} {
		if got := normalizeTotpCode(in); got != want {
			t.Errorf("normalizeTotpCode(%q) = %q, want %q", in, got, want)
		}
	}
}

// The exact round trip login depends on: a code generated from the stored
// secret at the current time must validate, and a stale/foreign one must not.
func TestTotpRoundTrip(t *testing.T) {
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "Tico Pay", AccountName: "test@ticopay.cr"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	code, err := totp.GenerateCode(key.Secret(), time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if !totp.Validate(code, key.Secret()) {
		t.Fatalf("freshly generated code %q did not validate", code)
	}
	// A code from a different secret must fail.
	other, _ := totp.Generate(totp.GenerateOpts{Issuer: "Tico Pay", AccountName: "otro@ticopay.cr"})
	otherCode, _ := totp.GenerateCode(other.Secret(), time.Now())
	if otherCode != code && totp.Validate(otherCode, key.Secret()) {
		t.Fatalf("code from another secret validated")
	}
	// A code from far in the past must fail (window is ~±30s).
	stale, _ := totp.GenerateCode(key.Secret(), time.Now().Add(-10*time.Minute))
	if stale != code && totp.Validate(stale, key.Secret()) {
		t.Fatalf("10-minute-old code validated")
	}
}
