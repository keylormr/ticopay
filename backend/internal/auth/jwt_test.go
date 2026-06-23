package auth

import (
	"testing"
	"time"
)

func TestJWTRoundTripCarriesVersion(t *testing.T) {
	m := NewManager("test-secret-at-least-32-chars-long-xx", 15*time.Minute, time.Hour)
	access, refresh, err := m.Issue("user-1", 7)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	ac, err := m.Parse(access, AccessToken)
	if err != nil {
		t.Fatalf("Parse access: %v", err)
	}
	if ac.UserID != "user-1" || ac.Ver != 7 {
		t.Errorf("access claims = %+v, want uid=user-1 ver=7", ac)
	}
	rc, err := m.Parse(refresh, RefreshToken)
	if err != nil {
		t.Fatalf("Parse refresh: %v", err)
	}
	if rc.Ver != 7 {
		t.Errorf("refresh ver = %d, want 7", rc.Ver)
	}

	// Wrong type is rejected.
	if _, err := m.Parse(access, RefreshToken); err == nil {
		t.Error("access token validated as refresh")
	}
	// Tampered/foreign secret is rejected.
	other := NewManager("a-completely-different-secret-32-chars", 15*time.Minute, time.Hour)
	if _, err := other.Parse(access, AccessToken); err == nil {
		t.Error("token validated under a different secret")
	}
}
