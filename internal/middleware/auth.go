package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"examshield/internal/auth"
	"examshield/internal/httpx"
	"examshield/internal/models"
)

// StudentCookie is the HTTP-only cookie carrying the student session token.
const StudentCookie = "es_student"

// StaffPrincipal is the authenticated staff member for a request.
type StaffPrincipal struct {
	ID   int64
	Role models.Role
}

func (p StaffPrincipal) IsAdmin() bool { return p.Role == models.RoleAdmin }

// StaffAuth requires a valid staff access token in "Authorization: Bearer".
// Tokens are never accepted from query strings (they would leak into logs).
func StaffAuth(tokens *auth.Tokens) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		raw, ok := strings.CutPrefix(h, "Bearer ")
		if !ok || raw == "" {
			httpx.Abort(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "missing bearer token")
			return
		}
		claims, err := tokens.ParseStaff(raw)
		if err != nil {
			httpx.Abort(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "invalid or expired token")
			return
		}
		id, ok := parseID(claims.Subject)
		if !ok {
			httpx.Abort(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "invalid token subject")
			return
		}
		c.Set(httpx.KeyUser, StaffPrincipal{ID: id, Role: claims.Role})
		c.Next()
	}
}

// RequireRole allows only the listed roles.
func RequireRole(roles ...models.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := Staff(c)
		for _, r := range roles {
			if p.Role == r {
				c.Next()
				return
			}
		}
		httpx.Abort(c, http.StatusForbidden, httpx.CodeForbidden, "your role cannot perform this action")
	}
}

// Staff returns the authenticated staff principal (zero value if none).
func Staff(c *gin.Context) StaffPrincipal {
	if v, ok := c.Get(httpx.KeyUser); ok {
		if p, ok := v.(StaffPrincipal); ok {
			return p
		}
	}
	return StaffPrincipal{}
}

// StudentAuth requires the student session cookie and binds it to the device
// fingerprint the browser presents: a cookie copied to another machine fails.
func StudentAuth(tokens *auth.Tokens, deriveFP func(clientFP string) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := c.Cookie(StudentCookie)
		if err != nil || raw == "" {
			httpx.Abort(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "no exam session")
			return
		}
		claims, err := tokens.ParseStudent(raw)
		if err != nil {
			httpx.Abort(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "exam session expired")
			return
		}
		fp := deriveFP(c.GetHeader("X-Device-FP"))
		if fp == "" || fp != claims.DeviceFP {
			httpx.Abort(c, http.StatusUnauthorized, httpx.CodeDeviceMismatch, "this session belongs to another device")
			return
		}
		c.Set(httpx.KeyStudent, claims)
		c.Request = c.Request.WithContext(httpx.WithClientIP(c.Request.Context(), ClientIP(c)))
		c.Next()
	}
}

// Student returns the student claims set by StudentAuth.
func Student(c *gin.Context) *auth.StudentClaims {
	if v, ok := c.Get(httpx.KeyStudent); ok {
		if s, ok := v.(*auth.StudentClaims); ok {
			return s
		}
	}
	return nil
}

func parseID(s string) (int64, bool) {
	var n int64
	if s == "" || len(s) > 18 {
		return 0, false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		n = n*10 + int64(ch-'0')
	}
	return n, n > 0
}
