package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	appmw "examshield/internal/middleware"
	"examshield/internal/models"
	"examshield/internal/services"
)

type AuthHandler struct {
	authService *services.AuthService
}

func NewAuthHandler(authService *services.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
		appmw.WriteError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	result, err := h.authService.Login(r.Context(), req.Email, req.Password, clientIP(r))
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidCredentials):
			appmw.WriteError(w, http.StatusUnauthorized, "invalid email or password")
		case errors.Is(err, services.ErrAccountSuspended):
			appmw.WriteError(w, http.StatusForbidden, "account is suspended")
		case errors.Is(err, services.ErrAccountInactive):
			appmw.WriteError(w, http.StatusForbidden, "account is inactive")
		default:
			appmw.WriteError(w, http.StatusInternalServerError, "unable to process login")
		}
		return
	}

	appmw.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"access_token":  result.AccessToken,
		"refresh_token": result.RefreshToken,
		"user": map[string]interface{}{
			"id":        result.User.ID,
			"email":     result.User.Email,
			"full_name": result.User.FullName,
			"role":      result.User.Role,
		},
	})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		appmw.WriteError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}
	access, err := h.authService.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		appmw.WriteError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}
	appmw.WriteJSON(w, http.StatusOK, map[string]string{"access_token": access})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	claims, ok := appmw.ClaimsFromContext(r.Context())
	if !ok {
		appmw.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	_ = h.authService.Logout(r.Context(), claims.UserID, clientIP(r))
	appmw.WriteJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

type registerRequest struct {
	FullName string      `json:"full_name"`
	Email    string      `json:"email"`
	Password string      `json:"password"`
	Role     models.Role `json:"role"`
}

// Register allows SUPER_ADMIN/ADMIN to create new accounts. RBAC
// middleware restricts which roles may call this, and which target
// roles are permitted, at the route level.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	claims, ok := appmw.ClaimsFromContext(r.Context())
	if !ok {
		appmw.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appmw.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" || req.FullName == "" || req.Role == "" {
		appmw.WriteError(w, http.StatusBadRequest, "full_name, email, password and role are required")
		return
	}
	// Only SUPER_ADMIN may create ADMIN accounts; ADMIN may create
	// LECTURER/STUDENT accounts. Least-privilege enforced here.
	if req.Role == models.RoleAdmin && claims.Role != models.RoleSuperAdmin {
		appmw.WriteError(w, http.StatusForbidden, "only a super admin can create admin accounts")
		return
	}
	if req.Role == models.RoleSuperAdmin {
		appmw.WriteError(w, http.StatusForbidden, "super admin accounts cannot be created via this endpoint")
		return
	}

	user, err := h.authService.RegisterUser(r.Context(), claims.OrganizationID, req.FullName, req.Email, req.Password, req.Role)
	if err != nil {
		appmw.WriteError(w, http.StatusInternalServerError, "unable to create account")
		return
	}
	appmw.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"id": user.ID, "email": user.Email, "role": user.Role,
	})
}

func mustParseUUID(w http.ResponseWriter, s string) (uuid.UUID, bool) {
	id, err := uuid.Parse(s)
	if err != nil {
		appmw.WriteError(w, http.StatusBadRequest, "invalid id")
		return uuid.Nil, false
	}
	return id, true
}
