package httpapi

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

const maxBodyBytes = 1 << 20 // 1 MB is plenty for a polygon with thousands of points

// requestLogger logs one structured line per request.
func requestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			if r.URL.Path == "/health" {
				return // keep health checks out of the logs
			}
			log.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}

// recoverer turns a panic into a JSON 500 (with time_ns) instead of a
// dropped connection, and logs the stack trace.
func recoverer(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			defer func() {
				if rec := recover(); rec != nil {
					if rec == http.ErrAbortHandler {
						panic(rec)
					}
					log.Error("panic", "error", rec, "path", r.URL.Path, "stack", string(debug.Stack()))
					writeJSON(w, http.StatusInternalServerError,
						newError("internal_error", "Something went wrong on our side. Please try again."),
						time.Since(start))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// limitBody caps request bodies so a huge upload can't exhaust memory.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// requireAPIKey enforces the optional API key. It is only installed when
// API_KEY is set, so by default the API is open (the assessment checks the
// endpoints directly). The key can come from the X-API-Key header, or from
// the api_key query parameter for WebSockets (browsers can't set headers on
// WebSocket requests).
func requireAPIKey(key string) func(http.Handler) http.Handler {
	expected := []byte(key)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions || r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			got := r.Header.Get("X-API-Key")
			if got == "" {
				got = r.URL.Query().Get("api_key")
			}
			if subtle.ConstantTimeCompare([]byte(got), expected) != 1 {
				writeJSON(w, http.StatusUnauthorized,
					newError("unauthorized", "a valid API key is required in the X-API-Key header"),
					time.Since(start))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
