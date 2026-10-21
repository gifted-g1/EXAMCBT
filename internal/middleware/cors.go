package middleware

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)

// SameOriginOnly replaces permissive CORS. The SPA is served by this same
// server, so legitimate browser requests are same-origin; any cross-origin
// request carrying an Origin header for a state-changing method is refused.
// This also blocks CSRF against the cookie-authenticated student and refresh
// endpoints, alongside SameSite=Strict cookies.
func SameOriginOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}
		u, err := url.Parse(origin)
		if err == nil && u.Host == c.Request.Host {
			c.Next()
			return
		}
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
			// No CORS headers are emitted, so the browser hides the response.
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "CROSS_ORIGIN_FORBIDDEN"})
	}
}
