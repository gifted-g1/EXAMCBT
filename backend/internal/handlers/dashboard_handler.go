package handlers

import (
	"net/http"

	"github.com/gorilla/mux"

	appmw "examshield/internal/middleware"
	"examshield/internal/models"
	"examshield/internal/services"
)

type DashboardHandler struct {
	exams   *services.ExamService
	reports *services.ReportService
}

func NewDashboardHandler(exams *services.ExamService, reports *services.ReportService) *DashboardHandler {
	return &DashboardHandler{exams: exams, reports: reports}
}

// Summary returns the role-appropriate dashboard payload: active,
// upcoming and completed exams plus counts, for admins/lecturers.
func (h *DashboardHandler) Summary(w http.ResponseWriter, r *http.Request) {
	claims, ok := appmw.ClaimsFromContext(r.Context())
	if !ok {
		appmw.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	exams, err := h.exams.ListForOrganization(r.Context(), claims.OrganizationID)
	if err != nil {
		appmw.WriteError(w, http.StatusInternalServerError, "unable to load dashboard")
		return
	}

	var active, upcoming, completed []models.Exam
	for _, e := range exams {
		switch e.Status {
		case models.ExamStatusActive:
			active = append(active, e)
		case models.ExamStatusScheduled, models.ExamStatusPublished:
			upcoming = append(upcoming, e)
		case models.ExamStatusEnded, models.ExamStatusArchived:
			completed = append(completed, e)
		}
	}

	appmw.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"active_exams":    active,
		"upcoming_exams":  upcoming,
		"completed_exams": completed,
		"total_exams":     len(exams),
	})
}

func (h *DashboardHandler) ExamReport(w http.ResponseWriter, r *http.Request) {
	examID, ok := mustParseUUID(w, mux.Vars(r)["id"])
	if !ok {
		return
	}
	report, err := h.reports.GenerateExamReport(r.Context(), examID)
	if err != nil {
		appmw.WriteError(w, http.StatusInternalServerError, "unable to generate report")
		return
	}
	appmw.WriteJSON(w, http.StatusOK, report)
}
