package handlers

import (
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"examshield/internal/httpx"
	"examshield/internal/middleware"
	"examshield/internal/services"
)

// --- Student endpoints (no dashboard: everything the exam page needs) ---------

// Gate: GET /api/student/gate — runs after LANGate.ExamEntry.
func (h *Handlers) Gate(c *gin.Context) {
	e := middleware.ExamFromContext(c)
	c.JSON(http.StatusOK, h.Students.Gate(e))
}

// Verify: POST /api/student/verify (multipart: matric_no, consent, selfie, id_photo)
func (h *Handlers) Verify(c *gin.Context) {
	e := middleware.ExamFromContext(c)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2*services.MaxPhotoBytes+64<<10)
	// The selfie is optional here: the service requires it only when the
	// exam has face verification switched on.
	selfie, _ := readPart(c, "selfie")
	idPhoto, _ := readPart(c, "id_photo")
	consent, _ := strconv.ParseBool(c.PostForm("consent"))
	res, err := h.Students.Verify(c.Request.Context(), e, services.VerifyInput{
		MatricNo: c.PostForm("matric_no"), Selfie: selfie, IDPhoto: idPhoto, Consent: consent,
		ClientFP: c.GetHeader("X-Device-FP"), ClientIP: middleware.ClientIP(c).String(),
	})
	if err != nil {
		fail(c, err)
		return
	}
	h.setStudentCookie(c, res.Token, res.ExpiresAt)
	c.JSON(http.StatusOK, res)
}

func readPart(c *gin.Context, name string) ([]byte, error) {
	fh, err := c.FormFile(name)
	if err != nil {
		return nil, err
	}
	return readFile(fh, services.MaxPhotoBytes)
}

func readFile(fh *multipart.FileHeader, limit int64) ([]byte, error) {
	if fh.Size > limit {
		return nil, services.ErrBadMedia
	}
	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, limit))
}

// SessionState: GET /api/student/session — resume after reload / reconnect.
func (h *Handlers) SessionState(c *gin.Context) {
	st, err := h.Students.State(c.Request.Context(), middleware.Student(c).SessionID())
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, st)
}

// SaveAnswer: PUT /api/student/answers {"question_id":1,"response":"2"}
func (h *Handlers) SaveAnswer(c *gin.Context) {
	var req struct {
		QuestionID int64  `json:"question_id"`
		Response   string `json:"response"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, services.MaxAnswerBytes+1024)
	if err := c.ShouldBindJSON(&req); err != nil || req.QuestionID <= 0 {
		httpx.BadRequest(c, "question_id and response are required")
		return
	}
	at, err := h.Students.SaveAnswer(c.Request.Context(), middleware.Student(c), req.QuestionID, req.Response)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"saved_at": at})
}

// Heartbeat: POST /api/student/heartbeat
func (h *Handlers) Heartbeat(c *gin.Context) {
	claims := middleware.Student(c)
	exam, deadline, err := h.Students.Heartbeat(c.Request.Context(), claims)
	if err != nil {
		fail(c, err)
		return
	}
	// The token expires at the student's deadline. If staff extended the
	// exam, re-issue it so the student isn't logged out at the original end.
	if want := deadline.Add(h.Cfg.SubmitGrace); claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(want.Add(-time.Second)) {
		if tok, err := h.Tokens.IssueStudent(claims.SessionID(), claims.ExamID, claims.StudentID, claims.DeviceFP, want); err == nil {
			h.setStudentCookie(c, tok, want)
		}
	}
	c.JSON(http.StatusOK, gin.H{"server_time": time.Now().UTC(), "ends_at": deadline, "proctoring": exam.Proctoring.Resolve()})
}

// ClientEvent: POST /api/student/events {"type":"TAB_SWITCH","metadata":{...}}
func (h *Handlers) ClientEvent(c *gin.Context) {
	var req struct {
		Type     string         `json:"type"`
		Metadata map[string]any `json:"metadata"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "type is required")
		return
	}
	if err := h.Students.ReportEvent(c.Request.Context(), middleware.Student(c), req.Type, req.Metadata); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

// Frame: POST /api/student/frames (body: image/jpeg)
func (h *Handlers) Frame(c *gin.Context) { h.media(c, "frame", services.MaxFrameBytes) }

// Audio: POST /api/student/audio (body: PCM16LE 16 kHz mono)
func (h *Handlers) Audio(c *gin.Context) {
	h.media(c, "audio", services.AudioSampleRate*2*services.MaxAudioSeconds)
}

func (h *Handlers) media(c *gin.Context, kind string, limit int) {
	data, err := io.ReadAll(io.LimitReader(c.Request.Body, int64(limit)+1))
	if err != nil || len(data) > limit {
		fail(c, services.ErrBadMedia)
		return
	}
	if err := h.Students.IngestMedia(c.Request.Context(), middleware.Student(c), kind, data); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

// Submit: POST /api/student/submit
func (h *Handlers) Submit(c *gin.Context) {
	sess, err := h.Students.Submit(c.Request.Context(), middleware.Student(c))
	if err != nil {
		fail(c, err)
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: middleware.StudentCookie, Value: "", Path: "/api/student",
		MaxAge: -1, HttpOnly: true, Secure: h.Cfg.CookieSecure, SameSite: http.SameSiteStrictMode})
	c.JSON(http.StatusOK, gin.H{"status": sess.Status, "ended_at": sess.EndedAt})
}

func (h *Handlers) setStudentCookie(c *gin.Context, token string, expires time.Time) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: middleware.StudentCookie, Value: token, Path: "/api/student", Expires: expires,
		MaxAge: int(time.Until(expires).Seconds()), HttpOnly: true, Secure: h.Cfg.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}
