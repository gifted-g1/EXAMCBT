package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"examshield/internal/httpx"
	"examshield/internal/middleware"
	"examshield/internal/models"
)

// ListViolations: GET /api/staff/exams/:id/violations?limit=200 — alert feed.
func (h *Handlers) ListViolations(c *gin.Context) {
	e, ok := h.examFor(c, false)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	vs, err := h.Repo.ViolationsForExam(c.Request.Context(), e.ID, limit)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"violations": vs})
}

// ReviewViolation: POST /api/staff/exams/:id/violations/:vid/review {"status":"DISMISSED"}
func (h *Handlers) ReviewViolation(c *gin.Context) {
	e, ok := h.examFor(c, false)
	if !ok {
		return
	}
	vid, ok := idParam(c, "vid")
	if !ok {
		return
	}
	var req struct {
		Status models.ReviewStatus `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "status is required")
		return
	}
	score, err := h.Proctor.Review(c.Request.Context(), e.ID, vid, middleware.Staff(c).ID, req.Status)
	if err != nil {
		fail(c, err)
		return
	}
	h.audit(c, "violation_review", strconv.FormatInt(vid, 10), map[string]any{"status": req.Status})
	c.JSON(http.StatusOK, gin.H{"integrity_score": score})
}
