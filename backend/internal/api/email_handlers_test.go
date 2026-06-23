package api

import (
	"strings"
	"testing"
)

func TestGenTokenUniqueAndHashed(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		raw, hash, err := genToken()
		if err != nil {
			t.Fatalf("genToken: %v", err)
		}
		// Raw token is URL-safe (no '+', '/', '=' that would break a query param).
		if strings.ContainsAny(raw, "+/=") {
			t.Errorf("raw token %q has non-URL-safe chars", raw)
		}
		// Hash is deterministic and never equals the raw token.
		if hash == raw {
			t.Errorf("hash equals raw token")
		}
		if hashToken(raw) != hash {
			t.Errorf("hashToken not deterministic for %q", raw)
		}
		if len(hash) != 64 { // sha256 hex
			t.Errorf("hash len = %d, want 64", len(hash))
		}
		if seen[raw] {
			t.Fatalf("duplicate token generated: %q", raw)
		}
		seen[raw] = true
	}
}

func TestHashTokenDiffersPerInput(t *testing.T) {
	if hashToken("abc") == hashToken("abd") {
		t.Error("different inputs produced the same hash")
	}
}
