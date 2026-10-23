package services

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"time"

	"examshield/internal/models"
	"examshield/internal/realtime"
	"examshield/internal/repositories"
)

type ProctorService struct {
	repo     *repositories.Repo
	recorder *Recorder
	hub      *realtime.Hub
}

func NewProctorService(repo *repositories.Repo, recorder *Recorder, hub *realtime.Hub) *ProctorService {
	return &ProctorService{repo: repo, recorder: recorder, hub: hub}
}

// Review confirms or dismisses a violation and recomputes the session's
// integrity score from its non-dismissed violations.
func (s *ProctorService) Review(ctx context.Context, examID, violationID, reviewer int64, status models.ReviewStatus) (float64, error) {
	if status != models.ReviewConfirmed && status != models.ReviewDismissed && status != models.ReviewPending {
		return 0, invalid("status must be CONFIRMED, DISMISSED or PENDING")
	}
	var score float64
	var sessionID int64
	err := s.repo.Tx(ctx, func(tx *repositories.Repo) error {
		v, err := tx.ViolationByID(ctx, violationID)
		if err != nil {
			return err
		}
		sess, err := tx.SessionByIDForUpdate(ctx, v.SessionID)
		if err != nil {
			return err
		}
		if sess.ExamID != examID {
			return repositories.ErrNotFound
		}
		sessionID = sess.ID
		if err := tx.ReviewViolation(ctx, violationID, reviewer, status); err != nil {
			return err
		}
		vs, err := tx.ViolationsForSession(ctx, sess.ID)
		if err != nil {
			return err
		}
		score = IntegrityFrom(vs)
		return tx.SetIntegrity(ctx, sess.ID, score)
	})
	if err == nil {
		s.hub.Publish(examID, "review", map[string]any{"violation_id": violationID, "session_id": sessionID,
			"status": status, "integrity_score": score})
	}
	return score, err
}

// Flag lets a proctor record an observation manually.
func (s *ProctorService) Flag(ctx context.Context, sess *models.ExamSession, eventType string, sev models.Severity, note string, by int64) (*models.Violation, error) {
	if !sev.Valid() {
		return nil, invalid("invalid severity")
	}
	if eventType == "" || len(eventType) > 50 {
		eventType = "PROCTOR_NOTE"
	}
	return s.recorder.Record(ctx, ViolationInput{SessionID: sess.ID, ExamID: sess.ExamID, EventType: eventType,
		Severity: sev, Source: models.SourceProctor, Metadata: map[string]any{"note": truncate(note, 500), "by": by}})
}

// ReportRow is one student's line in the exam report.
type ReportRow struct {
	SessionID      int64      `json:"session_id"`
	MatricNo       string     `json:"matric_no"`
	FullName       string     `json:"full_name"`
	Status         string     `json:"status"`
	Score          *float64   `json:"score"`
	MaxScore       float64    `json:"max_score"`
	IntegrityScore float64    `json:"integrity_score"`
	FaceMatch      *float64   `json:"face_match_score"`
	Violations     int64      `json:"violations"`
	HighViolations int64      `json:"high_violations"`
	AutoSubmitted  bool       `json:"auto_submitted"`
	StartedAt      time.Time  `json:"started_at"`
	EndedAt        *time.Time `json:"ended_at"`
	NeedsMarking   bool       `json:"needs_manual_marking"`
}

type Report struct {
	Exam        *models.Exam `json:"exam"`
	GeneratedAt time.Time    `json:"generated_at"`
	Rows        []ReportRow  `json:"rows"`
	Summary     struct {
		Candidates     int     `json:"candidates"`
		Submitted      int     `json:"submitted"`
		AverageScore   float64 `json:"average_score"`
		AverageIntegr  float64 `json:"average_integrity"`
		FlaggedForHigh int     `json:"flagged_high"`
	} `json:"summary"`
}

func (s *ProctorService) Report(ctx context.Context, e *models.Exam) (*Report, error) {
	rows, err := s.repo.SessionsForExam(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	qs, err := s.repo.Questions(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	var max float64
	hasText := false
	marks := make(map[int64]float64, len(qs))
	for _, q := range qs {
		max += q.Marks
		marks[q.ID] = q.Marks
		if q.Kind == models.QuestionText {
			hasText = true
		}
	}
	rep := &Report{Exam: e, GeneratedAt: time.Now().UTC()}
	var sumScore, sumInt float64
	for _, r := range rows {
		rowMax := max // out of the student's own questions when they had a draw
		if r.QuestionIDs != nil {
			rowMax = 0
			for _, id := range r.QuestionIDs {
				rowMax += marks[id]
			}
		}
		rep.Rows = append(rep.Rows, ReportRow{SessionID: r.ID, MatricNo: r.MatricNo, FullName: r.FullName, Status: string(r.Status),
			Score: r.Score, MaxScore: rowMax, IntegrityScore: r.IntegrityScore, FaceMatch: r.FaceMatchScore,
			Violations: r.ViolationsCount, HighViolations: r.HighCount, AutoSubmitted: r.AutoSubmitted,
			StartedAt: r.StartedAt, EndedAt: r.EndedAt, NeedsMarking: hasText})
		if r.Score != nil {
			rep.Summary.Submitted++
			sumScore += *r.Score
		}
		sumInt += r.IntegrityScore
		if r.HighCount > 0 {
			rep.Summary.FlaggedForHigh++
		}
	}
	rep.Summary.Candidates = len(rows)
	if rep.Summary.Submitted > 0 {
		rep.Summary.AverageScore = round2(sumScore / float64(rep.Summary.Submitted))
	}
	if len(rows) > 0 {
		rep.Summary.AverageIntegr = round2(sumInt / float64(len(rows)))
	}
	return rep, nil
}

// WriteCSV renders the report as CSV, neutralising spreadsheet formulas.
func (r *Report) WriteCSV(w io.Writer) error {
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"matric_no", "full_name", "status", "score", "max_score", "integrity_score",
		"face_match_score", "violations", "high_violations", "auto_submitted", "started_at", "ended_at", "needs_manual_marking"})
	f := func(p *float64) string {
		if p == nil {
			return ""
		}
		return strconv.FormatFloat(*p, 'f', 2, 64)
	}
	for _, row := range r.Rows {
		ended := ""
		if row.EndedAt != nil {
			ended = row.EndedAt.Format(time.RFC3339)
		}
		_ = cw.Write([]string{csvSafe(row.MatricNo), csvSafe(row.FullName), row.Status, f(row.Score),
			fmt.Sprintf("%.2f", row.MaxScore), fmt.Sprintf("%.2f", row.IntegrityScore), f(row.FaceMatch),
			strconv.FormatInt(row.Violations, 10), strconv.FormatInt(row.HighViolations, 10),
			strconv.FormatBool(row.AutoSubmitted), row.StartedAt.Format(time.RFC3339), ended,
			strconv.FormatBool(row.NeedsMarking)})
	}
	cw.Flush()
	return cw.Error()
}

// csvSafe prevents CSV/formula injection when the file is opened in Excel.
func csvSafe(s string) string {
	if s != "" && (s[0] == '=' || s[0] == '+' || s[0] == '-' || s[0] == '@' || s[0] == '\t' || s[0] == '\r') {
		return "'" + s
	}
	return s
}

func round2(f float64) float64 { return float64(int64(f*100+0.5)) / 100 }
