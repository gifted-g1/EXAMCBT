package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"examshield/internal/models"
)

type ExamRepository struct {
	db *gorm.DB
}

func NewExamRepository(db *gorm.DB) *ExamRepository {
	return &ExamRepository{db: db}
}

func (r *ExamRepository) Create(ctx context.Context, e *models.Exam) error {
	return r.db.WithContext(ctx).Create(e).Error
}

func (r *ExamRepository) Update(ctx context.Context, e *models.Exam) error {
	return r.db.WithContext(ctx).Save(e).Error
}

func (r *ExamRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.Exam{}, "id = ?", id).Error
}

func (r *ExamRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.Exam, error) {
	var e models.Exam
	err := r.db.WithContext(ctx).
		Preload("Questions.Options").
		First(&e, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &e, err
}

func (r *ExamRepository) ListByOrganization(ctx context.Context, orgID uuid.UUID) ([]models.Exam, error) {
	var exams []models.Exam
	err := r.db.WithContext(ctx).Where("organization_id = ?", orgID).Order("created_at desc").Find(&exams).Error
	return exams, err
}

func (r *ExamRepository) ListByCreator(ctx context.Context, creatorID uuid.UUID) ([]models.Exam, error) {
	var exams []models.Exam
	err := r.db.WithContext(ctx).Where("created_by_user_id = ?", creatorID).Order("created_at desc").Find(&exams).Error
	return exams, err
}

func (r *ExamRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.ExamStatus) error {
	return r.db.WithContext(ctx).Model(&models.Exam{}).Where("id = ?", id).Update("status", status).Error
}

func (r *ExamRepository) SetLANServerInfo(ctx context.Context, id uuid.UUID, ip string, port int) error {
	return r.db.WithContext(ctx).Model(&models.Exam{}).Where("id = ?", id).
		Updates(map[string]interface{}{"lan_server_ip": ip, "lan_server_port": port}).Error
}

func (r *ExamRepository) AddQuestion(ctx context.Context, q *models.Question) error {
	return r.db.WithContext(ctx).Create(q).Error
}

func (r *ExamRepository) UpdateQuestion(ctx context.Context, q *models.Question) error {
	return r.db.WithContext(ctx).Save(q).Error
}

func (r *ExamRepository) DeleteQuestion(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.Question{}, "id = ?", id).Error
}

func (r *ExamRepository) ListQuestions(ctx context.Context, examID uuid.UUID) ([]models.Question, error) {
	var qs []models.Question
	err := r.db.WithContext(ctx).Preload("Options").Where("exam_id = ?", examID).Order("order_index asc").Find(&qs).Error
	return qs, err
}

// --- Attempts ---

// GetOrCreateAttempt returns the student's existing attempt for the
// exam, or creates one. The unique (exam_id, student_id) index on
// ExamAttempt guarantees this never produces a duplicate attempt even
// under concurrent requests.
func (r *ExamRepository) GetOrCreateAttempt(ctx context.Context, examID, studentID uuid.UUID) (*models.ExamAttempt, error) {
	var attempt models.ExamAttempt
	err := r.db.WithContext(ctx).Where("exam_id = ? AND student_id = ?", examID, studentID).First(&attempt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		attempt = models.ExamAttempt{ExamID: examID, StudentID: studentID, Status: models.AttemptNotStarted}
		if err := r.db.WithContext(ctx).Create(&attempt).Error; err != nil {
			return nil, err
		}
		return &attempt, nil
	}
	return &attempt, err
}

func (r *ExamRepository) FindAttempt(ctx context.Context, id uuid.UUID) (*models.ExamAttempt, error) {
	var a models.ExamAttempt
	err := r.db.WithContext(ctx).First(&a, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &a, err
}

func (r *ExamRepository) UpdateAttempt(ctx context.Context, a *models.ExamAttempt) error {
	return r.db.WithContext(ctx).Save(a).Error
}

func (r *ExamRepository) ListAttemptsByExam(ctx context.Context, examID uuid.UUID) ([]models.ExamAttempt, error) {
	var attempts []models.ExamAttempt
	err := r.db.WithContext(ctx).Where("exam_id = ?", examID).Find(&attempts).Error
	return attempts, err
}

// UpsertAnswer inserts or updates the student's answer for a question,
// relying on the unique (attempt_id, question_id) index.
func (r *ExamRepository) UpsertAnswer(ctx context.Context, ans *models.Answer) error {
	return r.db.WithContext(ctx).
		Where("attempt_id = ? AND question_id = ?", ans.AttemptID, ans.QuestionID).
		Assign(*ans).
		FirstOrCreate(ans).Error
}

func (r *ExamRepository) ListAnswers(ctx context.Context, attemptID uuid.UUID) ([]models.Answer, error) {
	var answers []models.Answer
	err := r.db.WithContext(ctx).Where("attempt_id = ?", attemptID).Find(&answers).Error
	return answers, err
}
