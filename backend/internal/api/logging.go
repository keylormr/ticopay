package api

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// Logger is the process-wide structured logger (JSON lines on stdout, which
// Render captures). Replaces chi's plaintext middleware.Logger.
var Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

// slogRequests logs one JSON line per request: method, path, status, bytes,
// duration, IP and the chi request id (correlates with error logs).
func slogRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"duration_ms", float64(time.Since(start).Microseconds()) / 1000,
			"ip", r.RemoteAddr,
			"request_id", middleware.GetReqID(r.Context()),
		}
		switch {
		case ww.Status() >= 500:
			Logger.Error("request", attrs...)
		case ww.Status() >= 400:
			Logger.Warn("request", attrs...)
		default:
			Logger.Info("request", attrs...)
		}
	})
}
