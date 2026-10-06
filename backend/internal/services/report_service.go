package services

import (
	"context"

	"github.com/google/uuid"

	"examshield/internal/models"
	"examshield/internal/repositories"
)

type ExamReport struct {
	ExamID           uuid.UUID `json:"exam_id"`
	TotalStudents    int       `json:"total_students"`
	Attendance       int       `json:"attendance"` // students who started
	Completed        int       `json:"completed"`
	CompletionRate   float64   `json:"completion_rate"`
	AverageScore     float64   `json:"average_score"`
	HighestScore     float64   `json:"highest_score"`
	LowestScore      float64   `json:"lowest_score"`
	AIEventCounts    map[models.Severity]int `json:"ai_event_counts"`
	SuspiciousEvents int       `json:"suspicious_events"`
}

type ReportService struct {
	exams *repositories.ExamRepository
	mon   *repositories.MonitoringRepository
}

func NewReportService(exams *repositories.ExamRepository, mon *repositories.MonitoringRepository) *ReportService {
	return &ReportService{exams: exams, mon: mon}
}

func (s *ReportService) GenerateExamReport(ctx context.Context, examID uuid.UUID) (*ExamReport, error) {
	attempts, err := s.exams.ListAttemptsByExam(ctx, examID)
	if err != nil {
		return nil, err
	}
	events, err := s.mon.ListAIEventsByExam(ctx, examID)
	if err != nil {
		return nil, err
	}

	report := &ExamReport{
		ExamID:        examID,
		TotalStudents: len(attempts),
		AIEventCounts: map[models.Severity]int{},
	}

	var scoreSum float64
	var scoreCount int
	first := true

	for _, a := range attempts {
		if a.Status == models.AttemptInProgress || a.Status == models.AttemptSubmitted {
			report.Attendance++
		}
		if a.Status == models.AttemptSubmitted {
			report.Completed++
		}
		if a.Score != nil {
			scoreSum += *a.Score
			scoreCount++
			if first || *a.Score > report.HighestScore {
				report.HighestScore = *a.Score
			}
			if first || *a.Score < report.LowestScore {
				report.LowestScore = *a.Score
			}
			first = false
		}
	}
	if report.TotalStudents > 0 {
		report.CompletionRate = float64(report.Completed) / float64(report.TotalStudents) * 100
	}
	if scoreCount > 0 {
		report.AverageScore = scoreSum / float64(scoreCount)
	}

	for _, e := range events {
		report.AIEventCounts[e.Severity]++
		if e.Severity == models.SeverityHigh || e.Severity == models.SeverityCritical {
			report.SuspiciousEvents++
		}
	}

	return report, nil
}
