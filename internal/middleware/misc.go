package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"examshield/internal/auth"
	"examshield/internal/httpx"
)

// RequestLog writes one structured line per request. It logs the path
// without its query string (which can carry PINs or WS tickets) and never
// logs bodies, matric numbers or names.
func RequestLog(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		rid := auth.RandomHex(6)
		c.Header("X-Request-ID", rid)
		c.Next()
		status := c.Writer.Status()
		attrs := []any{
			"rid", rid,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"dur_ms", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
		}
		if code, ok := c.Get(httpx.KeyErrCode); ok {
			attrs = append(attrs, "code", code)
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "err", c.Errors.Last().Error())
		}
		switch {
		case status >= 500:
			log.Error("http", attrs...)
		case status >= 400:
			log.Warn("http", attrs...)
		default:
			log.Debug("http", attrs...)
		}
	}
}

// SecurityHeaders sets conservative browser hardening headers. The CSP
// allows the camera/mic only for this origin and forbids framing.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(self), microphone=(self), geolocation=(), payment=()")
		h.Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data: blob:; media-src 'self' blob:; "+
				"style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; "+
				"frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		h.Set("Cache-Control", "no-store")
		c.Next()
	}
}
