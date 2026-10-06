package middleware

import (
	"net/http"

	"examshield/internal/models"
)

// RequireRole enforces that the authenticated user holds one of the
// allowed roles. It must run after Authenticate. Authorization
// decisions are made server-side only, from the verified JWT claims —
// never from any role field sent by the client.
func RequireRole(allowed ...models.Role) func(http.Handler) http.Handler {
	allowedSet := make(map[models.Role]bool, len(allowed))
	for _, r := range allowed {
		allowedSet[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				writeError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			if !allowedSet[claims.Role] {
				writeError(w, http.StatusForbidden, "insufficient permissions for this action")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
