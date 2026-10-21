package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"examshield/internal/httpx"
)

// RateLimit applies a fixed-window limit keyed by keyFn. If Redis is
// unavailable the request is allowed (fail-open) and the error logged: an
// exam hall losing Redis must not lock every student out mid-exam. Login
// throttling is enforced separately (fail-closed) in the auth service.
func RateLimit(l Limiter, name string, limit int, window time.Duration, keyFn func(*gin.Context) string, log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := keyFn(c)
		if key == "" {
			c.Next()
			return
		}
		ok, retry, err := l.Hit(c.Request.Context(), "rl:"+name+":"+key, limit, window)
		if err != nil {
			log.Error("rate limiter unavailable", "limiter", name, "err", err)
			c.Next()
			return
		}
		if !ok {
			c.Header("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
			httpx.Abort(c, http.StatusTooManyRequests, httpx.CodeRateLimited, "too many requests")
			return
		}
		c.Next()
	}
}

// ByIP keys a limiter on the gate-normalised client IP.
func ByIP(c *gin.Context) string { return ClientIP(c).String() }
