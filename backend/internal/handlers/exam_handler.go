package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	appmw "examshield/internal/middleware"
	"examshield/internal/models"
	"examshield/internal/services"
)

type ExamHandler struct {
	exams *services.ExamService
	audit *services.AuditService
}

func NewExamHandler(exams *services.ExamService, audit *services.AuditService) *ExamHandler {
	return &ExamHandler{exams: exams, audit: audit}
}

type createExamRequest struct {
	Title                   string          `json:"title"`
	Course                  string          `json:"course"`
	CourseCode              string          `json:"course_code"`
	DurationMinutes         int             `json:"duration_minutes"`
	StartAt                 string          `json:"start_at"`
	EndAt                   string          `json:"end_at"`
	Mode                    models.ExamMode `json:"mode"`
	RandomizeQuestions      bool            `json:"randomize_questions"`
	RandomizeAnswers        bool            `json:"randomize_answers"`
	AllowNavigation         bool            `json:"allow_navigation"`
	EnableAIMonitoring      bool            `json:"enable_ai_monitoring"`
	RequireFaceVerification bool            `json:"require_face_verification"`
	PassingScore            float64         `json:"passing_score"`
	Instructions            string          `json:"instructions"`
}

func (h *ExamHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := appmw.ClaimsFromContext(r.Context())
	if !ok {
		appmw.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var req createExamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		appmw.WriteError(w, http.StatusBadRequest, "a valid title is required")
		return
	}
	if req.Mode == "" {
		req.Mode = models.ExamModeOnline
	}

	exam := &models.Exam{
		OrganizationID:          claims.OrganizationID,
		CreatedByUserID:         claims.UserID,
		Title:                   req.Title,
		Course:                  req.Course,
		CourseCode:              req.CourseCode,
		DurationMinutes:         req.DurationMinutes,
		Mode:                    req.Mode,
		RandomizeQuestions:      req.RandomizeQuestions,
		RandomizeAnswers:        req.RandomizeAnswers,
		AllowNavigation:         req.AllowNavigation,
		EnableAIMonitoring:      req.EnableAIMonitoring,
		RequireFaceVerification: req.RequireFaceVerification,
		PassingScore:            req.PassingScore,
		Instructions:            req.Instructions,
	}
	if err := h.exams.CreateExam(r.Context(), exam); err != nil {
		appmw.WriteError(w, http.StatusInternalServerError, "unable to create exam")
		return
	}
	h.audit.Log(r.Context(), claims.UserID, claims.Role, "EXAM_CREATED", "exam", exam.ID.String(), clientIP(r), "")
	appmw.WriteJSON(w, http.StatusCreated, exam)
}

func (h *ExamHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := mustParseUUID(w, mux.Vars(r)["id"])
	if !ok {
		return
	}
	exam, err := h.exams.GetExam(r.Context(), id)
	if err != nil {
		appmw.WriteError(w, http.StatusNotFound, "exam not found")
		return
	}
	appmw.WriteJSON(w, http.StatusOK, exam)
}

func (h *ExamHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, ok := appmw.ClaimsFromContext(r.Context())
	if !ok {
		appmw.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	exams, err := h.exams.ListForOrganization(r.Context(), claims.OrganizationID)
	if err != nil {
		appmw.WriteError(w, http.StatusInternalServerError, "unable to list exams")
		return
	}
	appmw.WriteJSON(w, http.StatusOK, exams)
}

func (h *ExamHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := mustParseUUID(w, mux.Vars(r)["id"])
	if !ok {
		return
	}
	exam, err := h.exams.GetExam(r.Context(), id)
	if err != nil {
		appmw.WriteError(w, http.StatusNotFound, "exam not found")
		return
	}
	var req createExamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appmw.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	exam.Title = req.Title
	exam.Course = req.Course
	exam.CourseCode = req.CourseCode
	exam.DurationMinutes = req.DurationMinutes
	exam.RandomizeQuestions = req.RandomizeQuestions
	exam.RandomizeAnswers = req.RandomizeAnswers
	exam.AllowNavigation = req.AllowNavigation
	exam.EnableAIMonitoring = req.EnableAIMonitoring
	exam.RequireFaceVerification = req.RequireFaceVerification
	exam.PassingScore = req.PassingScore
	exam.Instructions = req.Instructions

	if err := h.exams.UpdateExam(r.Context(), exam); err != nil {
		appmw.WriteError(w, http.StatusInternalServerError, "unable to update exam")
		return
	}
	appmw.WriteJSON(w, http.StatusOK, exam)
}

func (h *ExamHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := mustParseUUID(w, mux.Vars(r)["id"])
	if !ok {
		return
	}
	if err := h.exams.DeleteExam(r.Context(), id); err != nil {
		appmw.WriteError(w, http.StatusInternalServerError, "unable to delete exam")
		return
	}
	appmw.WriteJSON(w, http.StatusOK, map[string]string{"message": "exam deleted"})
}

// --- Questions ---

type addQuestionRequest struct {
	Type        models.QuestionType `json:"type"`
	Text        string              `json:"text"`
	Points      float64             `json:"points"`
	OrderIndex  int                 `json:"order_index"`
	CorrectText string              `json:"correct_text"`
	Options     []struct {
		Text      string `json:"text"`
		IsCorrect bool   `json:"is_correct"`
	} `json:"options"`
}

func (h *ExamHandler) AddQuestion(w http.ResponseWriter, r *http.Request) {
	examID, ok := mustParseUUID(w, mux.Vars(r)["id"])
	if !ok {
		return
	}
	var req addQuestionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Text == "" || req.Type == "" {
		appmw.WriteError(w, http.StatusBadRequest, "type and text are required")
		return
	}
	q := &models.Question{
		ExamID:      examID,
		Type:        req.Type,
		Text:        req.Text,
		Points:      req.Points,
		OrderIndex:  req.OrderIndex,
		CorrectText: req.CorrectText,
	}
	for _, o := range req.Options {
		q.Options = append(q.Options, models.Option{Text: o.Text, IsCorrect: o.IsCorrect})
	}
	if err := h.exams.AddQuestion(r.Context(), q); err != nil {
		appmw.WriteError(w, http.StatusInternalServerError, "unable to add question")
		return
	}
	appmw.WriteJSON(w, http.StatusCreated, q)
}

func (h *ExamHandler) ListQuestions(w http.ResponseWriter, r *http.Request) {
	examID, ok := mustParseUUID(w, mux.Vars(r)["id"])
	if !ok {
		return
	}
	questions, err := h.exams.ListQuestions(r.Context(), examID)
	if err != nil {
		appmw.WriteError(w, http.StatusInternalServerError, "unable to list questions")
		return
	}
	appmw.WriteJSON(w, http.StatusOK, questions)
}

// --- Lifecycle ---

func (h *ExamHandler) Schedule(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, "EXAM_SCHEDULED", func(id uuid.UUID) (*models.Exam, error) {
		return h.exams.Schedule(r.Context(), id)
	})
}

func (h *ExamHandler) Publish(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, "EXAM_PUBLISHED", func(id uuid.UUID) (*models.Exam, error) {
		return h.exams.Publish(r.Context(), id)
	})
}

func (h *ExamHandler) Start(w http.ResponseWriter, r *http.Request) {
	claims, ok := appmw.ClaimsFromContext(r.Context())
	if !ok {
		appmw.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id, ok := mustParseUUID(w, mux.Vars(r)["id"])
	if !ok {
		return
	}
	exam, lanInfo, err := h.exams.Start(r.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrInvalidTransition) {
			appmw.WriteError(w, http.StatusConflict, "exam cannot be started from its current status")
			return
		}
		appmw.WriteError(w, http.StatusInternalServerError, "unable to start exam")
		return
	}
	h.audit.Log(r.Context(), claims.UserID, claims.Role, "EXAM_STARTED", "exam", id.String(), clientIP(r), "")
	appmw.WriteJSON(w, http.StatusOK, map[string]interface{}{"exam": exam, "lan_server": lanInfo})
}

func (h *ExamHandler) End(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, "EXAM_ENDED", func(id uuid.UUID) (*models.Exam, error) {
		return h.exams.End(r.Context(), id)
	})
}

func (h *ExamHandler) transition(w http.ResponseWriter, r *http.Request, auditAction string, fn func(uuid.UUID) (*models.Exam, error)) {
	claims, ok := appmw.ClaimsFromContext(r.Context())
	if !ok {
		appmw.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id, ok := mustParseUUID(w, mux.Vars(r)["id"])
	if !ok {
		return
	}
	exam, err := fn(id)
	if err != nil {
		if errors.Is(err, services.ErrInvalidTransition) {
			appmw.WriteError(w, http.StatusConflict, "invalid status transition for this exam")
			return
		}
		appmw.WriteError(w, http.StatusInternalServerError, "unable to update exam status")
		return
	}
	h.audit.Log(r.Context(), claims.UserID, claims.Role, auditAction, "exam", id.String(), clientIP(r), "")
	appmw.WriteJSON(w, http.StatusOK, exam)
}

// --- Attempts ---

func (h *ExamHandler) StartAttempt(w http.ResponseWriter, r *http.Request) {
	claims, ok := appmw.ClaimsFromContext(r.Context())
	if !ok {
		appmw.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	examID, ok := mustParseUUID(w, mux.Vars(r)["id"])
	if !ok {
		return
	}
	attempt, err := h.exams.StartAttempt(r.Context(), examID, claims.UserID, clientIP(r))
	if err != nil {
		switch {
		case errors.Is(err, services.ErrAttemptNotEligible):
			appmw.WriteError(w, http.StatusConflict, "exam is not currently active")
		case errors.Is(err, services.ErrDuplicateAttempt):
			appmw.WriteError(w, http.StatusConflict, "you have already submitted this exam")
		default:
			appmw.WriteError(w, http.StatusInternalServerError, "unable to start attempt")
		}
		return
	}
	appmw.WriteJSON(w, http.StatusOK, attempt)
}

type submitAnswerRequest struct {
	AttemptID        string  `json:"attempt_id"`
	QuestionID       string  `json:"question_id"`
	SelectedOptionID *string `json:"selected_option_id,omitempty"`
	TextAnswer       string  `json:"text_answer,omitempty"`
}

func (h *ExamHandler) SubmitAnswer(w http.ResponseWriter, r *http.Request) {
	var req submitAnswerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appmw.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	attemptID, ok := mustParseUUID(w, req.AttemptID)
	if !ok {
		return
	}
	questionID, ok := mustParseUUID(w, req.QuestionID)
	if !ok {
		return
	}
	answer := &models.Answer{
		AttemptID:  attemptID,
		QuestionID: questionID,
		TextAnswer: req.TextAnswer,
		AnsweredAt: time.Now(),
	}
	if req.SelectedOptionID != nil {
		optID, err := uuid.Parse(*req.SelectedOptionID)
		if err == nil {
			answer.SelectedOptionID = &optID
		}
	}
	if err := h.exams.RecordAnswer(r.Context(), answer); err != nil {
		appmw.WriteError(w, http.StatusInternalServerError, "unable to record answer")
		return
	}
	appmw.WriteJSON(w, http.StatusOK, map[string]string{"message": "answer recorded"})
}

func (h *ExamHandler) SubmitExam(w http.ResponseWriter, r *http.Request) {
	claims, ok := appmw.ClaimsFromContext(r.Context())
	if !ok {
		appmw.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	attemptID, ok := mustParseUUID(w, mux.Vars(r)["attemptId"])
	if !ok {
		return
	}
	attempt, err := h.exams.SubmitAttempt(r.Context(), attemptID)
	if err != nil {
		if errors.Is(err, services.ErrDuplicateAttempt) {
			appmw.WriteError(w, http.StatusConflict, "this exam has already been submitted")
			return
		}
		appmw.WriteError(w, http.StatusInternalServerError, "unable to submit exam")
		return
	}
	h.audit.Log(r.Context(), claims.UserID, claims.Role, "EXAM_SUBMITTED", "exam_attempt", attemptID.String(), clientIP(r), "")
	appmw.WriteJSON(w, http.StatusOK, attempt)
}
