package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"examshield/internal/auth"
	"examshield/internal/httpx"
	"examshield/internal/middleware"
	"examshield/internal/models"
)

// ListUsers: GET /api/staff/users (admin)
func (h *Handlers) ListUsers(c *gin.Context) {
	users, err := h.Repo.ListUsers(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

// CreateUser: POST /api/staff/users (admin)
func (h *Handlers) CreateUser(c *gin.Context) {
	var req struct {
		Username string      `json:"username"`
		Password string      `json:"password"`
		FullName string      `json:"full_name"`
		Role     models.Role `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "invalid payload")
		return
	}
	name := strings.Join(strings.Fields(req.FullName), " ")
	if !auth.ValidUsername(req.Username) || !req.Role.Valid() || len(name) < 2 || len(name) > 120 {
		httpx.BadRequest(c, "username must be 8 digits, role proctor|admin, full_name 2–120 chars")
		return
	}
	hash, err := h.Auth.HashPassword(req.Password)
	if err != nil {
		httpx.BadRequest(c, err.Error())
		return
	}
	if _, err := h.Repo.UserByUsername(c.Request.Context(), req.Username); err == nil {
		httpx.Abort(c, http.StatusConflict, httpx.CodeConflict, "username already exists")
		return
	}
	u := &models.User{Username: req.Username, PasswordHash: hash, FullName: name, Role: req.Role, IsActive: true}
	if err := h.Repo.CreateUser(c.Request.Context(), u); err != nil {
		fail(c, err)
		return
	}
	h.audit(c, "user_create", u.Username, map[string]any{"role": u.Role})
	c.JSON(http.StatusCreated, u)
}

// UpdateUser: PATCH /api/staff/users/:id (admin) {is_active?, role?, password?, full_name?}
func (h *Handlers) UpdateUser(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		IsActive *bool       `json:"is_active"`
		Role     models.Role `json:"role"`
		Password string      `json:"password"`
		FullName string      `json:"full_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "invalid payload")
		return
	}
	ctx := c.Request.Context()
	target, err := h.Repo.UserByID(ctx, id)
	if err != nil {
		fail(c, err)
		return
	}
	fields := map[string]any{}
	revoke := false
	if req.FullName != "" {
		fields["full_name"] = strings.Join(strings.Fields(req.FullName), " ")
	}
	if req.Role != "" {
		if !req.Role.Valid() {
			httpx.BadRequest(c, "role must be proctor or admin")
			return
		}
		fields["role"] = req.Role
	}
	if req.IsActive != nil {
		fields["is_active"] = *req.IsActive
		revoke = !*req.IsActive
	}
	if req.Password != "" {
		hash, err := h.Auth.HashPassword(req.Password)
		if err != nil {
			httpx.BadRequest(c, err.Error())
			return
		}
		fields["password_hash"] = hash
		revoke = true
	}
	if len(fields) == 0 {
		httpx.BadRequest(c, "nothing to update")
		return
	}
	// Never let the last active admin be demoted or deactivated.
	demoting := (req.Role != "" && req.Role != models.RoleAdmin) || (req.IsActive != nil && !*req.IsActive)
	if target.Role == models.RoleAdmin && target.IsActive && demoting {
		n, err := h.Repo.CountActiveAdmins(ctx)
		if err != nil {
			fail(c, err)
			return
		}
		if n <= 1 {
			httpx.Abort(c, http.StatusConflict, httpx.CodeConflict, "cannot remove the last active admin")
			return
		}
	}
	if id == middleware.Staff(c).ID && demoting {
		httpx.Abort(c, http.StatusConflict, httpx.CodeConflict, "you cannot demote or deactivate yourself")
		return
	}
	if err := h.Repo.UpdateUser(ctx, id, fields); err != nil {
		fail(c, err)
		return
	}
	if revoke {
		_ = h.Repo.RevokeUserTokens(ctx, id)
	}
	delete(fields, "password_hash")
	h.audit(c, "user_update", target.Username, map[string]any{"fields": keys(fields), "password_changed": req.Password != ""})
	u, _ := h.Repo.UserByID(ctx, id)
	c.JSON(http.StatusOK, u)
}

// Audit: GET /api/staff/audit (admin)
func (h *Handlers) AuditLog(c *gin.Context) {
	rows, err := h.Repo.ListAudit(c.Request.Context(), 500)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"entries": rows})
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
