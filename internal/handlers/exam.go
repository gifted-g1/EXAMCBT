package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"examshield/internal/examimport"
	"examshield/internal/httpx"
	"examshield/internal/middleware"
	"examshield/internal/models"
	"examshield/internal/services"
)

// ListExams: GET /api/staff/exams?mine=1
func (h *Handlers) ListExams(c *gin.Context) {
	var owner int64
	if c.Query("mine") == "1" {
		owner = middleware.Staff(c).ID
	}
	exams, err := h.Repo.ListExams(c.Request.Context(), owner)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"exams": exams})
}

// CreateExam: POST /api/staff/exams
func (h *Handlers) CreateExam(c *gin.Context) {
	var in services.ExamInput
	if err := c.ShouldBindJSON(&in); err != nil {
		httpx.BadRequest(c, "invalid exam payload (dates must be RFC 3339)")
		return
	}
	e, err := h.Exams.Create(c.Request.Context(), middleware.Staff(c).ID, in)
	if err != nil {
		fail(c, err)
		return
	}
	h.audit(c, "exam_create", strconv.FormatInt(e.ID, 10), map[string]any{"title": e.Title})
	c.JSON(http.StatusCreated, e)
}

// GetExam: GET /api/staff/exams/:id
func (h *Handlers) GetExam(c *gin.Context) {
	e, ok := h.examFor(c, false)
	if !ok {
		return
	}
	qs, err := h.Repo.Questions(c.Request.Context(), e.ID)
	if err != nil {
		fail(c, err)
		return
	}
	e.QuestionsCount = int64(len(qs))
	p := middleware.Staff(c)
	c.JSON(http.StatusOK, gin.H{"exam": e, "can_manage": services.CanManage(p.ID, p.IsAdmin(), e)})
}

// UpdateExam: PUT /api/staff/exams/:id
func (h *Handlers) UpdateExam(c *gin.Context) {
	e, ok := h.examFor(c, true)
	if !ok {
		return
	}
	var in services.ExamInput
	if err := c.ShouldBindJSON(&in); err != nil {
		httpx.BadRequest(c, "invalid exam payload")
		return
	}
	out, err := h.Exams.Update(c.Request.Context(), e, in)
	if err != nil {
		fail(c, err)
		return
	}
	h.audit(c, "exam_update", strconv.FormatInt(e.ID, 10), nil)
	c.JSON(http.StatusOK, out)
}

// DeleteExam: DELETE /api/staff/exams/:id
func (h *Handlers) DeleteExam(c *gin.Context) {
	e, ok := h.examFor(c, true)
	if !ok {
		return
	}
	if err := h.Exams.Delete(c.Request.Context(), e); err != nil {
		fail(c, err)
		return
	}
	h.audit(c, "exam_delete", strconv.FormatInt(e.ID, 10), map[string]any{"title": e.Title})
	c.Status(http.StatusNoContent)
}

// ExamStatus: POST /api/staff/exams/:id/status {"action":"schedule|open|close"}
func (h *Handlers) ExamStatus(c *gin.Context) {
	e, ok := h.examFor(c, true)
	if !ok {
		return
	}
	var req struct {
		Action string `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "action is required")
		return
	}
	out, err := h.Exams.Transition(c.Request.Context(), e.ID, req.Action)
	if err != nil {
		fail(c, err)
		return
	}
	h.audit(c, "exam_"+req.Action, strconv.FormatInt(e.ID, 10), nil)
	c.JSON(http.StatusOK, out)
}

// ExamPIN: POST /api/staff/exams/:id/pin {"enabled":true} rotates; false clears.
func (h *Handlers) ExamPIN(c *gin.Context) {
	e, ok := h.examFor(c, true)
	if !ok {
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "enabled is required")
		return
	}
	out, err := h.Exams.SetPIN(c.Request.Context(), e, req.Enabled)
	if err != nil {
		fail(c, err)
		return
	}
	h.audit(c, "exam_pin", strconv.FormatInt(e.ID, 10), map[string]any{"enabled": req.Enabled})
	c.JSON(http.StatusOK, out)
}

// AccessCard: GET /api/staff/exams/:id/access-card
func (h *Handlers) AccessCard(c *gin.Context) {
	e, ok := h.examFor(c, false)
	if !ok {
		return
	}
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	card, err := h.Exams.AccessCard(e, c.Request.Host, scheme)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, card)
}

// ListQuestions: GET /api/staff/exams/:id/questions (includes answers; staff only)
func (h *Handlers) ListQuestions(c *gin.Context) {
	e, ok := h.examFor(c, false)
	if !ok {
		return
	}
	qs, err := h.Repo.Questions(c.Request.Context(), e.ID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"questions": qs})
}

// AddQuestions: POST /api/staff/exams/:id/questions — one object or {"questions":[...]}
func (h *Handlers) AddQuestions(c *gin.Context) {
	e, ok := h.examFor(c, true)
	if !ok {
		return
	}
	var bulk struct {
		Questions []services.QuestionInput `json:"questions"`
		services.QuestionInput
	}
	if err := c.ShouldBindJSON(&bulk); err != nil {
		httpx.BadRequest(c, "invalid question payload")
		return
	}
	ins := bulk.Questions
	if len(ins) == 0 {
		ins = []services.QuestionInput{bulk.QuestionInput}
	}
	qs, err := h.Exams.AddQuestions(c.Request.Context(), e, ins)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"questions": qs})
}

// UpdateQuestion: PUT /api/staff/exams/:id/questions/:qid
func (h *Handlers) UpdateQuestion(c *gin.Context) {
	e, ok := h.examFor(c, true)
	if !ok {
		return
	}
	qid, ok := idParam(c, "qid")
	if !ok {
		return
	}
	var in services.QuestionInput
	if err := c.ShouldBindJSON(&in); err != nil {
		httpx.BadRequest(c, "invalid question payload")
		return
	}
	if err := h.Exams.UpdateQuestion(c.Request.Context(), e, qid, in); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// DeleteQuestion: DELETE /api/staff/exams/:id/questions/:qid
func (h *Handlers) DeleteQuestion(c *gin.Context) {
	e, ok := h.examFor(c, true)
	if !ok {
		return
	}
	qid, ok := idParam(c, "qid")
	if !ok {
		return
	}
	if err := h.Exams.DeleteQuestion(c.Request.Context(), e, qid); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Report: GET /api/staff/exams/:id/report[?format=csv]
func (h *Handlers) Report(c *gin.Context) {
	e, ok := h.examFor(c, false)
	if !ok {
		return
	}
	rep, err := h.Proctor.Report(c.Request.Context(), e)
	if err != nil {
		fail(c, err)
		return
	}
	if c.Query("format") == "csv" {
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", `attachment; filename="exam-`+strconv.FormatInt(e.ID, 10)+`-report.csv"`)
		if err := rep.WriteCSV(c.Writer); err != nil {
			h.Log.Error("report csv write", "err", err)
		}
		return
	}
	c.JSON(http.StatusOK, rep)
}

// PurgeMedia: POST /api/staff/exams/:id/purge-media (admin, closed exams only)
func (h *Handlers) PurgeMedia(c *gin.Context) {
	e, ok := h.examFor(c, true)
	if !ok {
		return
	}
	if e.Status != models.ExamClosed {
		fail(c, services.ErrInvalidState)
		return
	}
	if err := h.Scheduler.PurgeExamMedia(c.Request.Context(), e.ID); err != nil {
		fail(c, err)
		return
	}
	h.audit(c, "exam_purge_media", strconv.FormatInt(e.ID, 10), nil)
	c.Status(http.StatusNoContent)
}

// System: GET /api/staff/system — network + AI diagnostics for the dashboard.
func (h *Handlers) System(c *gin.Context) {
	card := gin.H{"mode": h.Cfg.Mode, "ai_backends": h.Engine.Backends(),
		"face_match_threshold": h.Cfg.FaceMatchThreshold, "face_match_assurance": h.Matcher.Assurance(),
		"redis_embedded": h.Redis.Embedded(), "retention_days": h.Cfg.RetentionDays,
		"client_ip": middleware.ClientIP(c).String()}
	c.JSON(http.StatusOK, card)
}

// ParseQuestionsPDF: POST /api/staff/exams/:id/questions/parse-pdf
// (multipart "file"). Extracts and parses a question paper and returns a
// preview; nothing is saved. The dashboard lets staff fix missing answers
// and then imports through AddQuestions.
func (h *Handlers) ParseQuestionsPDF(c *gin.Context) {
	e, ok := h.examFor(c, true)
	if !ok {
		return
	}
	if e.Status == models.ExamActive || e.Status == models.ExamClosed {
		fail(c, services.ErrInvalidState)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, examimport.MaxPDFBytes+64<<10)
	fh, err := c.FormFile("file")
	if err != nil {
		httpx.BadRequest(c, "PDF file is required (field 'file', max 20 MB)")
		return
	}
	data, err := readFile(fh, examimport.MaxPDFBytes)
	if err != nil {
		httpx.BadRequest(c, "PDF file is larger than 20 MB")
		return
	}
	lines, pages, err := examimport.ExtractLines(data)
	if err != nil {
		switch {
		case errors.Is(err, examimport.ErrNotPDF), errors.Is(err, examimport.ErrEncrypted),
			errors.Is(err, examimport.ErrNoText), errors.Is(err, examimport.ErrTooLarge):
			httpx.Abort(c, http.StatusUnprocessableEntity, "PDF_UNREADABLE", err.Error())
		default:
			httpx.Internal(c, err)
		}
		return
	}
	res := examimport.Parse(lines)
	res.Pages = pages
	h.audit(c, "questions_pdf_parsed", strconv.FormatInt(e.ID, 10), map[string]any{"file": fh.Filename, "questions": len(res.Questions)})
	c.JSON(http.StatusOK, res)
}

// SampleQuestionsPDF: GET /api/staff/questions/sample.pdf — an example paper
// in the format the importer understands.
func (h *Handlers) SampleQuestionsPDF(c *gin.Context) {
	c.Header("Content-Disposition", `attachment; filename="exam-shield-sample-questions.pdf"`)
	c.Data(http.StatusOK, "application/pdf", examimport.SamplePDF())
}

// SetProctoring: PUT /api/staff/exams/:id/proctoring (owner or admin)
// Body: any of face_verification, camera_monitoring, audio_monitoring,
// fullscreen_required, tab_switch_detection, clipboard_blocking. Omitted
// switches keep their current value.
func (h *Handlers) SetProctoring(c *gin.Context) {
	e, ok := h.examFor(c, true)
	if !ok {
		return
	}
	var patch models.ProctoringSettings
	if err := c.ShouldBindJSON(&patch); err != nil {
		httpx.BadRequest(c, "body must be a JSON object of true/false switches")
		return
	}
	merged := e.Proctoring
	for _, f := range []struct{ dst, src **bool }{
		{&merged.FaceVerification, &patch.FaceVerification},
		{&merged.CameraMonitoring, &patch.CameraMonitoring},
		{&merged.AudioMonitoring, &patch.AudioMonitoring},
		{&merged.FullscreenRequired, &patch.FullscreenRequired},
		{&merged.TabSwitchDetection, &patch.TabSwitchDetection},
		{&merged.ClipboardBlocking, &patch.ClipboardBlocking},
	} {
		if *f.src != nil {
			*f.dst = *f.src
		}
	}
	before := e.Proctoring.Resolve()
	out, err := h.Exams.SetProctoring(c.Request.Context(), e, merged.Resolve())
	if err != nil {
		fail(c, err)
		return
	}
	h.audit(c, "exam_proctoring", strconv.FormatInt(e.ID, 10), map[string]any{"before": before, "after": out.Proctoring.Resolve()})
	c.JSON(http.StatusOK, out)
}
