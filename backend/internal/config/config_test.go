package config

import (
	"strings"
	"testing"
)

// IsProd must fail CLOSED: only explicit dev markers disable hardening; a blank
// or misspelled APP_ENV lands on the production path.
func TestIsProdFailsClosed(t *testing.T) {
	cases := map[string]bool{
		"development": false,
		"dev":         false,
		"test":        false,
		"local":       false,
		"Development": false,
		"  dev  ":     false,
		"production":  true,
		"PRODUCTION":  true,
		"prod":        true, // misspelling → prod
		"staging":     true,
		"":            true, // blank → prod
		"whatever":    true,
	}
	for env, want := range cases {
		if got := (Config{AppEnv: env}).IsProd(); got != want {
			t.Errorf("IsProd(%q) = %v, want %v", env, got, want)
		}
	}
}

func TestSecretIsWeak(t *testing.T) {
	weak := []string{"", DefaultJWTSecret, "short", strings.Repeat("a", 31)}
	for _, s := range weak {
		if !(Config{JWTSecret: s}).SecretIsWeak() {
			t.Errorf("SecretIsWeak(%q) = false, want true", s)
		}
	}
	if (Config{JWTSecret: strings.Repeat("a", 32)}).SecretIsWeak() {
		t.Error("SecretIsWeak(32-char secret) = true, want false")
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV("a, b ,,c ")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("splitCSV len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("splitCSV[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	// Empty input falls back to the local dev origin (one element).
	if fb := splitCSV(""); len(fb) != 1 {
		t.Errorf("splitCSV(%q) = %v, want a single fallback origin", "", fb)
	}
}

// Load() must fail closed on a forgotten APP_ENV: this exercises the real boot
// path (env default), not Config{} built directly, which is where the fail-open
// hid before.
func TestLoadFailsClosedWithoutAppEnv(t *testing.T) {
	t.Setenv("APP_ENV", "") // a deploy that forgot the variable
	t.Setenv("SEED_DEMO", "true")
	c := Load()
	if !c.IsProd() {
		t.Error("Load() con APP_ENV ausente debe caer en producción (fail-closed)")
	}
	if c.SeedDemo {
		t.Error("Load() debe forzar SeedDemo=false cuando IsProd (defensa en profundidad)")
	}
}

func TestLoadDevExplicitKeepsSeed(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("SEED_DEMO", "true")
	c := Load()
	if c.IsProd() {
		t.Error("APP_ENV=development debe ser dev, no prod")
	}
	if !c.SeedDemo {
		t.Error("SeedDemo debe respetarse en desarrollo explícito")
	}
}
