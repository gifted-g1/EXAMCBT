package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"examshield/internal/httpx"
	"examshield/internal/middleware"
	"examshield/internal/services"
)

const (
	refreshCookie     = "es_refresh"
	refreshCookiePath = "/api/staff/auth"
)

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Login: POST /api/staff/auth/login
func (h *Handlers) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "username and password are required")
		return
	}
	res, err := h.Auth.Login(c.Request.Context(), req.Username, req.Password, middleware.ClientIP(c).String())
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) || errors.Is(err, services.ErrLoginThrottled) {
			_ = h.Repo.Audit(c.Request.Context(), nil, "login_failed", req.Username, middleware.ClientIP(c).String(), nil)
		}
		fail(c, err)
		return
	}
	h.setRefresh(c, res.RefreshToken, res.RefreshExp)
	c.Set(httpx.KeyUser, middleware.StaffPrincipal{ID: res.User.ID, Role: res.User.Role})
	h.audit(c, "login", res.User.Username, nil)
	c.JSON(http.StatusOK, res)
}

// Refresh: POST /api/staff/auth/refresh (HTTP-only cookie)
func (h *Handlers) Refresh(c *gin.Context) {
	raw, _ := c.Cookie(refreshCookie)
	res, err := h.Auth.Refresh(c.Request.Context(), raw)
	if err != nil {
		h.clearRefresh(c)
		fail(c, err)
		return
	}
	h.setRefresh(c, res.RefreshToken, res.RefreshExp)
	c.JSON(http.StatusOK, res)
}

// Logout: POST /api/staff/auth/logout
func (h *Handlers) Logout(c *gin.Context) {
	raw, _ := c.Cookie(refreshCookie)
	if err := h.Auth.Logout(c.Request.Context(), raw); err != nil {
		h.Log.Error("logout revoke failed", "err", err)
	}
	h.clearRefresh(c)
	c.Status(http.StatusNoContent)
}

// Me: GET /api/staff/me
func (h *Handlers) Me(c *gin.Context) {
	u, err := h.Repo.UserByID(c.Request.Context(), middleware.Staff(c).ID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *Handlers) setRefresh(c *gin.Context, token string, exp time.Time) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: refreshCookie, Value: token, Path: refreshCookiePath, Expires: exp,
		MaxAge: int(time.Until(exp).Seconds()), HttpOnly: true, Secure: h.Cfg.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *Handlers) clearRefresh(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{Name: refreshCookie, Value: "", Path: refreshCookiePath, MaxAge: -1,
		HttpOnly: true, Secure: h.Cfg.CookieSecure, SameSite: http.SameSiteStrictMode})
}
