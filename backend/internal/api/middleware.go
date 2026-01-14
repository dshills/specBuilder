package api

import (
	"log"
	"net/http"
	"strings"
	"time"
)

// CORSConfig holds CORS configuration.
type CORSConfig struct {
	// AllowedOrigins is a comma-separated list of allowed origins.
	// Use "*" to allow all origins (not recommended for production).
	AllowedOrigins string
}

// CORS adds CORS headers with configurable allowed origins.
// For production, set AllowedOrigins to specific domains.
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	allowedOrigins := cfg.AllowedOrigins
	if allowedOrigins == "" {
		allowedOrigins = "*"
	}

	// Parse origins into a set for O(1) lookup
	var originSet map[string]bool
	allowAll := allowedOrigins == "*"
	if !allowAll {
		originSet = make(map[string]bool)
		for _, origin := range strings.Split(allowedOrigins, ",") {
			originSet[strings.TrimSpace(origin)] = true
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Determine which origin to allow
			var allowOrigin string
			if allowAll {
				allowOrigin = "*"
			} else if origin != "" && originSet[origin] {
				allowOrigin = origin
			}

			if allowOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
				if !allowAll {
					w.Header().Set("Vary", "Origin")
				}
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Logger logs HTTP requests.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.status, time.Since(start))
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher to support SSE streaming.
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
