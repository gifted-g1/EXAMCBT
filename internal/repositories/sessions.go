package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"examshield/internal/models"
)

func (r *Repo) SessionByID(ctx context.Context, id int64) (*models.ExamSession, error) {
	var s models.ExamSession
	if err := r.q(ctx).Preload("Student").First(&s, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

func (r *Repo) SessionByIDForUpdate(ctx context.Context, id int64) (*models.ExamSession, error) {
	var s models.ExamSession
	if err := r.forUpdate(r.q(ctx)).First(&s, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

func (r *Repo) SessionByExamStudentForUpdate(ctx context.Context, examID, studentID int64) (*models.ExamSession, error) {
	var s models.ExamSession
	if err := r.forUpdate(r.q(ctx)).Where("exam_id = ? AND student_id = ?", examID, studentID).First(&s).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

func (r *Repo) CreateSession(ctx context.Context, s *models.ExamSession) error {
	return r.q(ctx).Create(s).Error
}

func (r *Repo) UpdateSession(ctx context.Context, id int64, fields map[string]any) error {
	return r.q(ctx).Model(&models.ExamSession{}).Where("id = ?", id).Updates(fields).Error
}

func (r *Repo) TouchSession(ctx context.Context, id int64, at time.Time) error {
	return r.q(ctx).Model(&models.ExamSession{}).Where("id = ?", id).UpdateColumn("last_seen_at", at).Error
}

// SessionRow is one tile of the proctor live grid.
type SessionRow struct {
	models.ExamSession
	MatricNo        string `json:"matric_no"`
	FullName        string `json:"full_name"`
	ViolationsCount int64  `json:"violations_count"`
	HighCount       int64  `json:"high_count"`
	AnsweredCount   int64  `json:"answered_count"`
}

// SessionsForExam returns every session of an exam with student info and
// aggregate counts in a fixed number of queries (no N+1).
func (r *Repo) SessionsForExam(ctx context.Context, examID int64) ([]SessionRow, error) {
	var rows []SessionRow
	err := r.q(ctx).Table("exam_sessions AS s").
		Select("s.*, st.matric_no, st.full_name").
		Joins("JOIN students st ON st.id = s.student_id").
		Where("s.exam_id = ?", examID).
		Order("st.matric_no").
		Scan(&rows).Error
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	ids := make([]int64, len(rows))
	idx := make(map[int64]int, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
		idx[rows[i].ID] = i
	}
	type vc struct {
		SessionID int64
		N         int64
		High      int64
	}
	var vcs []vc
	if err := r.q(ctx).Model(&models.Violation{}).
		Select("session_id, COUNT(*) AS n, SUM(CASE WHEN severity IN ('HIGH','CRITICAL') THEN 1 ELSE 0 END) AS high").
		Where("session_id IN ? AND review_status <> ?", ids, models.ReviewDismissed).
		Group("session_id").Scan(&vcs).Error; err != nil {
		return nil, err
	}
	for _, v := range vcs {
		rows[idx[v.SessionID]].ViolationsCount = v.N
		rows[idx[v.SessionID]].HighCount = v.High
	}
	type ac struct {
		SessionID int64
		N         int64
	}
	var acs []ac
	if err := r.q(ctx).Model(&models.Answer{}).Select("session_id, COUNT(*) AS n").
		Where("session_id IN ? AND response <> ''", ids).Group("session_id").Scan(&acs).Error; err != nil {
		return nil, err
	}
	for _, a := range acs {
		rows[idx[a.SessionID]].AnsweredCount = a.N
	}
	return rows, nil
}

// OpenSessionsForExam returns INIT/ACTIVE sessions, for auto-submission.
func (r *Repo) OpenSessionsForExam(ctx context.Context, examID int64) ([]models.ExamSession, error) {
	var out []models.ExamSession
	err := r.q(ctx).Where("exam_id = ? AND status IN ?", examID, []models.SessionStatus{models.SessionInit, models.SessionActive}).Find(&out).Error
	return out, err
}

func (r *Repo) SessionsForStudent(ctx context.Context, studentID int64) ([]models.ExamSession, error) {
	var out []models.ExamSession
	err := r.q(ctx).Where("student_id = ?", studentID).Find(&out).Error
	return out, err
}

// --- Answers -----------------------------------------------------------------

// UpsertAnswer saves a response, relying on uq_session_q for idempotency.
func (r *Repo) UpsertAnswer(ctx context.Context, a *models.Answer) error {
	return r.q(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "session_id"}, {Name: "question_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"response", "saved_at"}),
	}).Create(a).Error
}

func (r *Repo) Answers(ctx context.Context, sessionID int64) ([]models.Answer, error) {
	var out []models.Answer
	err := r.q(ctx).Where("session_id = ?", sessionID).Find(&out).Error
	return out, err
}

// --- Violations & findings ----------------------------------------------------

func (r *Repo) CreateViolation(ctx context.Context, v *models.Violation) error {
	return r.q(ctx).Create(v).Error
}

func (r *Repo) CreateFindings(ctx context.Context, f []models.AIFinding) error {
	if len(f) == 0 {
		return nil
	}
	return r.q(ctx).Create(&f).Error
}

// AdjustIntegrity subtracts delta from a session's integrity score atomically
// (clamped at 0) and returns the new value.
func (r *Repo) AdjustIntegrity(ctx context.Context, sessionID int64, delta float64) (float64, error) {
	err := r.q(ctx).Model(&models.ExamSession{}).Where("id = ?", sessionID).
		UpdateColumn("integrity_score", gorm.Expr("CASE WHEN integrity_score - ? < 0 THEN 0 ELSE integrity_score - ? END", delta, delta)).Error
	if err != nil {
		return 0, err
	}
	var score float64
	err = r.q(ctx).Model(&models.ExamSession{}).Where("id = ?", sessionID).Pluck("integrity_score", &score).Error
	return score, err
}

func (r *Repo) SetIntegrity(ctx context.Context, sessionID int64, score float64) error {
	return r.q(ctx).Model(&models.ExamSession{}).Where("id = ?", sessionID).UpdateColumn("integrity_score", score).Error
}

func (r *Repo) ViolationsForSession(ctx context.Context, sessionID int64) ([]models.Violation, error) {
	var out []models.Violation
	err := r.q(ctx).Where("session_id = ?", sessionID).Order("occurred_at DESC").Find(&out).Error
	return out, err
}

// ViolationRow is a violation with its student, for the alert feed.
type ViolationRow struct {
	models.Violation
	ExamID   int64  `json:"exam_id"`
	MatricNo string `json:"matric_no"`
	FullName string `json:"full_name"`
}

func (r *Repo) ViolationsForExam(ctx context.Context, examID int64, limit int) ([]ViolationRow, error) {
	var out []ViolationRow
	err := r.q(ctx).Table("violations AS v").
		Select("v.*, s.exam_id, st.matric_no, st.full_name").
		Joins("JOIN exam_sessions s ON s.id = v.session_id").
		Joins("JOIN students st ON st.id = s.student_id").
		Where("s.exam_id = ?", examID).
		Order("v.occurred_at DESC, v.id DESC").Limit(limit).
		Scan(&out).Error
	return out, err
}

func (r *Repo) ViolationByID(ctx context.Context, id int64) (*models.Violation, error) {
	var v models.Violation
	if err := r.q(ctx).First(&v, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &v, nil
}

func (r *Repo) ReviewViolation(ctx context.Context, id, reviewer int64, status models.ReviewStatus) error {
	now := time.Now().UTC()
	return r.q(ctx).Model(&models.Violation{}).Where("id = ?", id).
		Updates(map[string]any{"review_status": status, "reviewed_by": reviewer, "reviewed_at": now}).Error
}

func (r *Repo) FindingsForSession(ctx context.Context, sessionID int64, limit int) ([]models.AIFinding, error) {
	var out []models.AIFinding
	err := r.q(ctx).Where("session_id = ?", sessionID).Order("id DESC").Limit(limit).Find(&out).Error
	return out, err
}

// PurgeExamMedia clears AI findings and media references for an exam and
// stamps media_purged_at. Violations themselves are retained as the record.
func (r *Repo) PurgeExamMedia(ctx context.Context, examID int64) error {
	return r.Tx(ctx, func(tx *Repo) error {
		sub := tx.q(ctx).Model(&models.ExamSession{}).Select("id").Where("exam_id = ?", examID)
		if err := tx.q(ctx).Where("session_id IN (?)", sub).Delete(&models.AIFinding{}).Error; err != nil {
			return err
		}
		if err := tx.q(ctx).Model(&models.Violation{}).Where("session_id IN (?)", sub).Update("media_ref", nil).Error; err != nil {
			return err
		}
		if err := tx.q(ctx).Model(&models.ExamSession{}).Where("exam_id = ?", examID).
			Updates(map[string]any{"reference_key": "", "id_photo_key": ""}).Error; err != nil {
			return err
		}
		return tx.q(ctx).Model(&models.Exam{}).Where("id = ?", examID).Update("media_purged_at", time.Now().UTC()).Error
	})
}

func (r *Repo) SessionByExamStudent(ctx context.Context, examID, studentID int64) (*models.ExamSession, error) {
	var s models.ExamSession
	if err := r.q(ctx).Where("exam_id = ? AND student_id = ?", examID, studentID).First(&s).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}
