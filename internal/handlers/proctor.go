package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"examshield/internal/auth"
	"examshield/internal/httpx"
	"examshield/internal/middleware"
	"examshield/internal/models"
	"examshield/internal/services"
	"examshield/internal/store"
)

// ListSessions: GET /api/staff/exams/:id/sessions — the live grid.
func (h *Handlers) ListSessions(c *gin.Context) {
	e, ok := h.examFor(c, false)
	if !ok {
		return
	}
	rows, err := h.Repo.SessionsForExam(c.Request.Context(), e.ID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"exam": e, "sessions": rows, "server_time": time.Now().UTC()})
}

// sessionFor loads :sid; manage requires ownership of its exam.
func (h *Handlers) sessionFor(c *gin.Context, manage bool) (*models.ExamSession, bool) {
	sid, ok := idParam(c, "sid")
	if !ok {
		return nil, false
	}
	sess, err := h.Repo.SessionByID(c.Request.Context(), sid)
	if err != nil {
		fail(c, err)
		return nil, false
	}
	if manage {
		e, err := h.Repo.ExamByID(c.Request.Context(), sess.ExamID)
		if err != nil {
			fail(c, err)
			return nil, false
		}
		p := middleware.Staff(c)
		if !services.CanManage(p.ID, p.IsAdmin(), e) {
			fail(c, services.ErrForbidden)
			return nil, false
		}
	}
	return sess, true
}

// SessionDetail: GET /api/staff/sessions/:sid
func (h *Handlers) SessionDetail(c *gin.Context) {
	sess, ok := h.sessionFor(c, false)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	vs, err := h.Repo.ViolationsForSession(ctx, sess.ID)
	if err != nil {
		fail(c, err)
		return
	}
	fs, err := h.Repo.FindingsForSession(ctx, sess.ID, 50)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"session": sess, "violations": vs, "findings": fs,
		"has_reference": sess.ReferenceKey != "", "has_id_photo": sess.IDPhotoKey != ""})
}

// SessionImage: GET /api/staff/sessions/:sid/image/:which (latest|reference|id)
func (h *Handlers) SessionImage(c *gin.Context) {
	sess, ok := h.sessionFor(c, false)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	var key string
	switch c.Param("which") {
	case "latest":
		k, err := h.Redis.Get(ctx, services.LatestFrameKey(sess.ID)).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			fail(c, err)
			return
		}
		key = k
	case "reference":
		key = sess.ReferenceKey
	case "id":
		key = sess.IDPhotoKey
	default:
		httpx.NotFound(c, "image")
		return
	}
	h.serveBlob(c, key)
}

// Media: GET /api/staff/media?key=frames/1/2/123.jpg — evidence viewer for
// a violation's media_ref. Only evidence prefixes are served.
func (h *Handlers) Media(c *gin.Context) {
	key := c.Query("key")
	if !(strings.HasPrefix(key, "frames/") || strings.HasPrefix(key, "verify/") || strings.HasPrefix(key, "roster/")) {
		httpx.NotFound(c, "media")
		return
	}
	h.serveBlob(c, key)
}

func (h *Handlers) serveBlob(c *gin.Context, key string) {
	if key == "" || !store.ValidKey(key) {
		httpx.NotFound(c, "image")
		return
	}
	data, err := h.Blobs.Get(c.Request.Context(), key)
	if errors.Is(err, store.ErrBlobNotFound) {
		httpx.NotFound(c, "image")
		return
	}
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Data(http.StatusOK, http.DetectContentType(data), data)
}

// UnlockSession: POST /api/staff/sessions/:sid/unlock
func (h *Handlers) UnlockSession(c *gin.Context) {
	sess, ok := h.sessionFor(c, true)
	if !ok {
		return
	}
	if err := h.Students.Unlock(c.Request.Context(), sess); err != nil {
		fail(c, err)
		return
	}
	h.audit(c, "session_unlock", strconv.FormatInt(sess.ID, 10), nil)
	h.Hub.Publish(sess.ExamID, "session", gin.H{"session_id": sess.ID, "event": "unlocked"})
	c.Status(http.StatusNoContent)
}

// TerminateSession: POST /api/staff/sessions/:sid/terminate
func (h *Handlers) TerminateSession(c *gin.Context) {
	sess, ok := h.sessionFor(c, true)
	if !ok {
		return
	}
	out, err := h.Students.Terminate(c.Request.Context(), sess.ID)
	if err != nil {
		fail(c, err)
		return
	}
	h.audit(c, "session_terminate", strconv.FormatInt(sess.ID, 10), nil)
	c.JSON(http.StatusOK, out)
}

// FlagSession: POST /api/staff/sessions/:sid/flag {"event_type","severity","note"}
func (h *Handlers) FlagSession(c *gin.Context) {
	sess, ok := h.sessionFor(c, false)
	if !ok {
		return
	}
	var req struct {
		EventType string          `json:"event_type"`
		Severity  models.Severity `json:"severity"`
		Note      string          `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "severity is required")
		return
	}
	v, err := h.Proctor.Flag(c.Request.Context(), sess, strings.ToUpper(req.EventType), req.Severity, req.Note, middleware.Staff(c).ID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, v)
}

// --- WebSocket ---------------------------------------------------------------

const wsTicketTTL = 30 * time.Second

// WSTicket: POST /api/staff/ws-ticket {"exam_id":1} → one-time ticket. The
// browser WebSocket API cannot send an Authorization header, and putting the
// JWT in the URL would leak it into proxy logs, so a 30 s single-use ticket
// is exchanged instead.
func (h *Handlers) WSTicket(c *gin.Context) {
	var req struct {
		ExamID int64 `json:"exam_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ExamID <= 0 {
		httpx.BadRequest(c, "exam_id is required")
		return
	}
	if _, err := h.Repo.ExamByID(c.Request.Context(), req.ExamID); err != nil {
		fail(c, err)
		return
	}
	ticket := auth.RandomHex(24)
	val := strconv.FormatInt(req.ExamID, 10) + ":" + strconv.FormatInt(middleware.Staff(c).ID, 10)
	if err := h.Redis.Set(c.Request.Context(), "ws_ticket:"+ticket, val, wsTicketTTL).Err(); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ticket": ticket, "expires_in": int(wsTicketTTL.Seconds())})
}

// WS: GET /api/staff/ws?ticket=...
func (h *Handlers) WS(shutdown context.Context) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticket := c.Query("ticket")
		if len(ticket) != 48 {
			httpx.Abort(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "missing ticket")
			return
		}
		// GETDEL makes the ticket single-use atomically.
		val, err := h.Redis.GetDel(c.Request.Context(), "ws_ticket:"+ticket).Result()
		if err != nil {
			httpx.Abort(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "invalid or used ticket")
			return
		}
		examStr, _, _ := strings.Cut(val, ":")
		examID, err := strconv.ParseInt(examStr, 10, 64)
		if err != nil {
			httpx.Abort(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "invalid ticket")
			return
		}
		h.Hub.Serve(shutdown, c.Writer, c.Request, examID)
	}
}

// AnswerScript: GET /api/staff/sessions/:sid/answers — one student's marked
// answer sheet (their answer, the correct answer, marks per question).
func (h *Handlers) AnswerScript(c *gin.Context) {
	sess, ok := h.sessionFor(c, false)
	if !ok {
		return
	}
	script, err := h.Proctor.AnswerScript(c.Request.Context(), sess.ID)
	if err != nil {
		fail(c, err)
		return
	}
	h.audit(c, "answer_script_view", strconv.FormatInt(sess.ID, 10), nil)
	c.JSON(http.StatusOK, script)
}

// ItemAnalysis: GET /api/staff/exams/:id/item-analysis — how the class did
// on each question (right / wrong / skipped, most chosen wrong option).
func (h *Handlers) ItemAnalysis(c *gin.Context) {
	e, ok := h.examFor(c, false)
	if !ok {
		return
	}
	stats, candidates, err := h.Proctor.ItemAnalysis(c.Request.Context(), e)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"candidates": candidates, "questions": stats})
}
