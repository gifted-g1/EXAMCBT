package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"examshield/internal/httpx"
	"examshield/internal/middleware"
	"examshield/internal/services"
)

// MarkingOverview: GET /api/staff/exams/:id/marking — written questions
// with their marking guides and how many answers are marked.
func (h *Handlers) MarkingOverview(c *gin.Context) {
	e, ok := h.examFor(c, false)
	if !ok {
		return
	}
	ov, err := h.Marking.Overview(c.Request.Context(), e)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, ov)
}

// SetMarkingGuide: PUT /api/staff/exams/:id/questions/:qid/guide
func (h *Handlers) SetMarkingGuide(c *gin.Context) {
	e, ok := h.examFor(c, true)
	if !ok {
		return
	}
	qid, ok := idParam(c, "qid")
	if !ok {
		return
	}
	var in services.GuideInput
	if err := c.ShouldBindJSON(&in); err != nil {
		httpx.BadRequest(c, "body must be {points: [{point, marks, keywords}], model_answer}")
		return
	}
	q, err := h.Marking.SetGuide(c.Request.Context(), e, qid, in)
	if err != nil {
		fail(c, err)
		return
	}
	h.audit(c, "marking_guide_set", strconv.FormatInt(qid, 10), map[string]any{"exam_id": e.ID, "points": len(in.Points)})
	c.JSON(http.StatusOK, q)
}

// MarkingScripts: GET /api/staff/exams/:id/marking/:qid — every written
// answer to one question, anonymised, with suggested marks.
func (h *Handlers) MarkingScripts(c *gin.Context) {
	e, ok := h.examFor(c, false)
	if !ok {
		return
	}
	qid, ok := idParam(c, "qid")
	if !ok {
		return
	}
	out, err := h.Marking.Scripts(c.Request.Context(), e, qid)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// AISuggest: POST /api/staff/exams/:id/marking/:qid/ai-suggest
// {answer_ids?: [...]} — asks the AI for suggested marks. Off unless the
// server has AI_MARKING=true and ANTHROPIC_API_KEY.
func (h *Handlers) AISuggest(c *gin.Context) {
	e, ok := h.examFor(c, true)
	if !ok {
		return
	}
	qid, ok := idParam(c, "qid")
	if !ok {
		return
	}
	var in struct {
		AnswerIDs []int64 `json:"answer_ids"`
	}
	_ = c.ShouldBindJSON(&in)
	res, err := h.Marking.SuggestAI(c.Request.Context(), e, qid, in.AnswerIDs)
	if err != nil {
		fail(c, err)
		return
	}
	h.audit(c, "ai_mark_suggest", strconv.FormatInt(qid, 10), map[string]any{"exam_id": e.ID,
		"requested": res.Requested, "suggested": res.Suggested, "failed": res.Failed})
	c.JSON(http.StatusOK, res)
}

// MarkAnswer: PUT /api/staff/exams/:id/answers/:aid/mark {marks, comment}
// — the examiner's decision. marks: null clears the mark.
func (h *Handlers) MarkAnswer(c *gin.Context) {
	e, ok := h.examFor(c, true)
	if !ok {
		return
	}
	aid, ok := idParam(c, "aid")
	if !ok {
		return
	}
	var in struct {
		Marks   *float64 `json:"marks"`
		Comment string   `json:"comment"`
		Source  string   `json:"source"` // manual | keyword | ai — which suggestion was accepted, for the audit log
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		httpx.BadRequest(c, "body must be {marks, comment}")
		return
	}
	res, err := h.Marking.Mark(c.Request.Context(), e, aid, in.Marks, in.Comment, middleware.Staff(c).ID)
	if err != nil {
		fail(c, err)
		return
	}
	h.audit(c, "answer_marked", strconv.FormatInt(aid, 10), map[string]any{"exam_id": e.ID, "marks": in.Marks, "source": in.Source})
	c.JSON(http.StatusOK, res)
}
