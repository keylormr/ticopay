package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"tuanispay/backend/internal/config"
)

func fireMetrics(a *App, token string) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	r.Use(withLang)
	r.Get("/metrics", a.handleMetrics)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestMetricsDisabledWithoutToken(t *testing.T) {
	a := &App{cfg: config.Config{}} // METRICS_TOKEN unset → endpoint disabled
	if rec := fireMetrics(a, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("metrics with no token configured: %d, want 404", rec.Code)
	}
	if rec := fireMetrics(a, "anything"); rec.Code != http.StatusNotFound {
		t.Fatalf("metrics with no token configured (token sent): %d, want 404", rec.Code)
	}
}

func TestMetricsRequiresToken(t *testing.T) {
	const token = "s3cr3t-token-abcdefghij"
	a := &App{cfg: config.Config{MetricsToken: token}}

	if rec := fireMetrics(a, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("metrics without auth: %d, want 401", rec.Code)
	}
	if rec := fireMetrics(a, "wrong-token"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("metrics with wrong token: %d, want 401", rec.Code)
	}

	rec := fireMetrics(a, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics with valid token: %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"tuanispay_uptime_seconds", "tuanispay_http_requests_total", "tuanispay_goroutines"} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics body missing %q:\n%s", want, body)
		}
	}
}
