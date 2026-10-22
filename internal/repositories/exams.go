package repositories

import (
	"context"
	"time"

	"examshield/internal/models"
)

func (r *Repo) CreateExam(ctx context.Context, e *models.Exam) error {
	return r.q(ctx).Create(e).Error
}

func (r *Repo) ExamByID(ctx context.Context, id int64) (*models.Exam, error) {
	var e models.Exam
	if err := r.q(ctx).First(&e, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &e, nil
}

func (r *Repo) ExamByIDForUpdate(ctx context.Context, id int64) (*models.Exam, error) {
	var e models.Exam
	if err := r.forUpdate(r.q(ctx)).First(&e, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &e, nil
}

// ListExams returns exams newest first with question counts in one query.
// createdBy=0 means all exams.
func (r *Repo) ListExams(ctx context.Context, createdBy int64) ([]models.Exam, error) {
	var out []models.Exam
	q := r.q(ctx).Order("starts_at DESC")
	if createdBy > 0 {
		q = q.Where("created_by = ?", createdBy)
	}
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	ids := make([]int64, len(out))
	for i := range out {
		ids[i] = out[i].ID
	}
	type row struct {
		ExamID int64
		N      int64
	}
	var counts []row
	if err := r.q(ctx).Model(&models.Question{}).Select("exam_id, COUNT(*) AS n").
		Where("exam_id IN ?", ids).Group("exam_id").Scan(&counts).Error; err != nil {
		return nil, err
	}
	m := make(map[int64]int64, len(counts))
	for _, c := range counts {
		m[c.ExamID] = c.N
	}
	for i := range out {
		out[i].QuestionsCount = m[out[i].ID]
	}
	return out, nil
}

func (r *Repo) UpdateExam(ctx context.Context, id int64, fields map[string]any) error {
	res := r.q(ctx).Model(&models.Exam{}).Where("id = ?", id).Updates(fields)
	if res.Error == nil && res.RowsAffected == 0 {
		return ErrNotFound
	}
	return res.Error
}

func (r *Repo) DeleteExam(ctx context.Context, id int64) error {
	res := r.q(ctx).Delete(&models.Exam{}, id)
	if res.Error == nil && res.RowsAffected == 0 {
		return ErrNotFound
	}
	return res.Error
}

// OpenExams returns ACTIVE exams whose window contains now. Uses
// idx_status_window.
func (r *Repo) OpenExams(ctx context.Context, now time.Time) ([]models.Exam, error) {
	var out []models.Exam
	err := r.q(ctx).Where("status = ? AND starts_at <= ? AND ends_at > ?", models.ExamActive, now, now).
		Order("starts_at").Find(&out).Error
	return out, err
}

// ExamsToActivate returns SCHEDULED exams whose start time has passed.
func (r *Repo) ExamsToActivate(ctx context.Context, now time.Time) ([]models.Exam, error) {
	var out []models.Exam
	err := r.q(ctx).Where("status = ? AND starts_at <= ? AND ends_at > ?", models.ExamScheduled, now, now).Find(&out).Error
	return out, err
}

// ExamsToClose returns ACTIVE/SCHEDULED exams whose window ended before cutoff.
func (r *Repo) ExamsToClose(ctx context.Context, cutoff time.Time) ([]models.Exam, error) {
	var out []models.Exam
	err := r.q(ctx).Where("status IN ? AND ends_at <= ?", []models.ExamStatus{models.ExamActive, models.ExamScheduled}, cutoff).Find(&out).Error
	return out, err
}

// ExamsForMediaPurge returns CLOSED exams that ended before cutoff and still
// have media.
func (r *Repo) ExamsForMediaPurge(ctx context.Context, cutoff time.Time) ([]models.Exam, error) {
	var out []models.Exam
	err := r.q(ctx).Where("status = ? AND ends_at < ? AND media_purged_at IS NULL", models.ExamClosed, cutoff).Find(&out).Error
	return out, err
}

// PinInUse reports whether another live (SCHEDULED/ACTIVE) exam uses pin.
// PINs must be unique among live exams because the PIN is what routes a
// student to the right exam when several share a subnet.
func (r *Repo) PinInUse(ctx context.Context, pin string, exceptExamID int64) (bool, error) {
	var n int64
	err := r.q(ctx).Model(&models.Exam{}).
		Where("pin_code = ? AND id <> ? AND status IN ?", pin, exceptExamID, []models.ExamStatus{models.ExamScheduled, models.ExamActive}).
		Count(&n).Error
	return n > 0, err
}

// --- Questions ---------------------------------------------------------------

func (r *Repo) Questions(ctx context.Context, examID int64) ([]models.Question, error) {
	var out []models.Question
	err := r.q(ctx).Where("exam_id = ?", examID).Order("position, id").Find(&out).Error
	return out, err
}

func (r *Repo) CreateQuestions(ctx context.Context, qs []models.Question) error {
	if len(qs) == 0 {
		return nil
	}
	return r.q(ctx).CreateInBatches(qs, 200).Error
}

func (r *Repo) NextQuestionPosition(ctx context.Context, examID int64) (int, error) {
	var max *int
	err := r.q(ctx).Model(&models.Question{}).Where("exam_id = ?", examID).Select("MAX(position)").Scan(&max).Error
	if err != nil || max == nil {
		return 1, err
	}
	return *max + 1, nil
}

func (r *Repo) UpdateQuestion(ctx context.Context, examID, id int64, q *models.Question) error {
	res := r.q(ctx).Model(&models.Question{}).Where("id = ? AND exam_id = ?", id, examID).
		Select("kind", "prompt", "options", "correct_option", "marks", "position").Updates(q)
	if res.Error == nil && res.RowsAffected == 0 {
		return ErrNotFound
	}
	return res.Error
}

func (r *Repo) DeleteQuestion(ctx context.Context, examID, id int64) error {
	res := r.q(ctx).Where("id = ? AND exam_id = ?", id, examID).Delete(&models.Question{})
	if res.Error == nil && res.RowsAffected == 0 {
		return ErrNotFound
	}
	return res.Error
}

// SetExamProctoring stores the per-exam proctoring switches. A struct update
// (not a map) is used so GORM applies the column's JSON serializer.
func (r *Repo) SetExamProctoring(ctx context.Context, id int64, p models.ProctoringSettings) error {
	res := r.q(ctx).Model(&models.Exam{ID: id}).Select("proctoring").Updates(&models.Exam{Proctoring: p})
	if res.Error == nil && res.RowsAffected == 0 {
		return ErrNotFound
	}
	return res.Error
}

// SetMarkingGuide saves a written question's marking guide and model answer.
func (r *Repo) SetMarkingGuide(ctx context.Context, q *models.Question) error {
	return r.q(ctx).Model(&models.Question{}).Where("id = ? AND exam_id = ?", q.ID, q.ExamID).
		Select("marking_guide", "model_answer").Updates(q).Error
}
