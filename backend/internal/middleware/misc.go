package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

// RequestLogger emits a structured log line per request: method, path,
// status, latency, and remote IP. No request/response bodies are
// logged, which keeps sensitive fields (passwords, tokens) out of logs.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"remote_ip", clientIP(r),
			)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// SecurityHeaders sets standard hardening headers on every response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-XSS-Protection", "0")
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}

// RateLimiter is a simple fixed-window, per-IP limiter suitable as a
// baseline safeguard (e.g. login endpoints). For multi-instance
// deployments this should be backed by Redis instead of memory.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	limit    int
	window   time.Duration
}

type bucket struct {
	count     int
	windowEnd time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{buckets: make(map[string]*bucket), limit: limit, window: window}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		rl.mu.Lock()
		b, ok := rl.buckets[ip]
		now := time.Now()
		if !ok || now.After(b.windowEnd) {
			b = &bucket{count: 0, windowEnd: now.Add(rl.window)}
			rl.buckets[ip] = b
		}
		b.count++
		exceeded := b.count > rl.limit
		rl.mu.Unlock()

		if exceeded {
			writeError(w, http.StatusTooManyRequests, "too many requests, please slow down")
			return
		}
		next.ServeHTTP(w, r)
	})
}
