package api

import "testing"

// Uses a private guard instance so tests don't pollute the global one.
func TestLoginGuardLockout(t *testing.T) {
	g := &loginGuard{fails: map[string]*attemptInfo{}}
	key := "test@ticopay.cr"

	for i := 0; i < maxLoginFails-1; i++ {
		g.fail(key)
		if g.locked(key) {
			t.Fatalf("locked after %d fails, want unlocked until %d", i+1, maxLoginFails)
		}
	}
	g.fail(key) // hits the threshold
	if !g.locked(key) {
		t.Fatalf("not locked after %d fails", maxLoginFails)
	}

	// Other keys are unaffected.
	if g.locked("otra@ticopay.cr") {
		t.Fatal("unrelated key reported locked")
	}

	// Reset forgives immediately (successful login path).
	g.reset(key)
	if g.locked(key) {
		t.Fatal("still locked after reset")
	}
}
