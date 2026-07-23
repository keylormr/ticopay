package api

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

// metricsState holds lightweight process counters exposed at /metrics in
// Prometheus text format. No external dependency; the request middleware
// updates the counters and handleMetrics renders them on demand.
type metricsState struct {
	startedAt        time.Time
	requests         atomic.Int64
	inflight         atomic.Int64
	s2xx             atomic.Int64
	s3xx             atomic.Int64
	s4xx             atomic.Int64
	s5xx             atomic.Int64
	ledgerDriftCents atomic.Int64 // total abs drift from the last reconciliation
}

var metrics = &metricsState{startedAt: time.Now()}

// recordRequest tallies one finished request by status class. A status of 0
// (handler wrote a body without an explicit code) counts as 2xx, matching the
// implicit 200 the net/http server sends.
func recordRequest(status int) {
	metrics.requests.Add(1)
	switch {
	case status >= 500:
		metrics.s5xx.Add(1)
	case status >= 400:
		metrics.s4xx.Add(1)
	case status >= 300:
		metrics.s3xx.Add(1)
	default:
		metrics.s2xx.Add(1)
	}
}

// metricsToken reads the bearer token from Authorization: Bearer or, as a
// convenience for scrapers, X-Metrics-Token.
func metricsToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	return strings.TrimSpace(r.Header.Get("X-Metrics-Token"))
}

// handleMetrics exposes process metrics in Prometheus text format, gated by
// METRICS_TOKEN with a constant-time comparison. With no token configured the
// endpoint is disabled (404) so internals aren't exposed by default.
func (a *App) handleMetrics(w http.ResponseWriter, r *http.Request) {
	want := a.cfg.MetricsToken
	if want == "" {
		http.NotFound(w, r)
		return
	}
	got := metricsToken(r)
	if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintf(w, "# HELP tuanispay_uptime_seconds Process uptime in seconds.\n# TYPE tuanispay_uptime_seconds gauge\ntuanispay_uptime_seconds %d\n",
		int64(time.Since(metrics.startedAt).Seconds()))
	fmt.Fprint(w, "# HELP tuanispay_http_requests_total HTTP requests by status class.\n# TYPE tuanispay_http_requests_total counter\n")
	fmt.Fprintf(w, "tuanispay_http_requests_total{class=\"2xx\"} %d\n", metrics.s2xx.Load())
	fmt.Fprintf(w, "tuanispay_http_requests_total{class=\"3xx\"} %d\n", metrics.s3xx.Load())
	fmt.Fprintf(w, "tuanispay_http_requests_total{class=\"4xx\"} %d\n", metrics.s4xx.Load())
	fmt.Fprintf(w, "tuanispay_http_requests_total{class=\"5xx\"} %d\n", metrics.s5xx.Load())
	fmt.Fprintf(w, "# HELP tuanispay_http_requests_inflight In-flight HTTP requests.\n# TYPE tuanispay_http_requests_inflight gauge\ntuanispay_http_requests_inflight %d\n", metrics.inflight.Load())
	fmt.Fprintf(w, "# HELP tuanispay_goroutines Current goroutines.\n# TYPE tuanispay_goroutines gauge\ntuanispay_goroutines %d\n", runtime.NumGoroutine())
	fmt.Fprintf(w, "# HELP tuanispay_mem_alloc_bytes Allocated heap bytes.\n# TYPE tuanispay_mem_alloc_bytes gauge\ntuanispay_mem_alloc_bytes %d\n", mem.Alloc)
	fmt.Fprintf(w, "# HELP tuanispay_ledger_drift_cents Total absolute drift between cached balances and the journal at the last reconciliation (should be 0).\n# TYPE tuanispay_ledger_drift_cents gauge\ntuanispay_ledger_drift_cents %d\n", metrics.ledgerDriftCents.Load())

	// DB pool saturation — nil-guarded so the endpoint (and its tests) work
	// without a live pool.
	if a.pool != nil {
		st := a.pool.Stat()
		fmt.Fprintf(w, "# HELP tuanispay_db_conns_total Total connections in the pool.\n# TYPE tuanispay_db_conns_total gauge\ntuanispay_db_conns_total %d\n", st.TotalConns())
		fmt.Fprintf(w, "# HELP tuanispay_db_conns_acquired Acquired (in-use) connections.\n# TYPE tuanispay_db_conns_acquired gauge\ntuanispay_db_conns_acquired %d\n", st.AcquiredConns())
	}
}
