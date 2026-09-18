package repositories

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"examshield/internal/models"
)

type MonitoringRepository struct {
	db *gorm.DB
}

func NewMonitoringRepository(db *gorm.DB) *MonitoringRepository {
	return &MonitoringRepository{db: db}
}

func (r *MonitoringRepository) SaveFaceVerification(ctx context.Context, f *models.FaceVerification) error {
	return r.db.WithContext(ctx).Create(f).Error
}

func (r *MonitoringRepository) LatestFaceVerification(ctx context.Context, studentID uuid.UUID) (*models.FaceVerification, error) {
	var f models.FaceVerification
	err := r.db.WithContext(ctx).Where("student_id = ?", studentID).Order("created_at desc").First(&f).Error
	return &f, err
}

func (r *MonitoringRepository) SaveAIEvent(ctx context.Context, e *models.AIEvent) error {
	return r.db.WithContext(ctx).Create(e).Error
}

func (r *MonitoringRepository) ListAIEventsByExam(ctx context.Context, examID uuid.UUID) ([]models.AIEvent, error) {
	var events []models.AIEvent
	err := r.db.WithContext(ctx).Where("exam_id = ?", examID).Order("timestamp asc").Find(&events).Error
	return events, err
}

func (r *MonitoringRepository) ListAIEventsByAttempt(ctx context.Context, attemptID uuid.UUID) ([]models.AIEvent, error) {
	var events []models.AIEvent
	err := r.db.WithContext(ctx).Where("attempt_id = ?", attemptID).Order("timestamp asc").Find(&events).Error
	return events, err
}

func (r *MonitoringRepository) MarkAIEventReviewed(ctx context.Context, id uuid.UUID, reviewerID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&models.AIEvent{}).Where("id = ?", id).
		Updates(map[string]interface{}{"reviewed": true, "reviewed_by": reviewerID}).Error
}

func (r *MonitoringRepository) SaveSecurityEvent(ctx context.Context, e *models.SecurityEvent) error {
	return r.db.WithContext(ctx).Create(e).Error
}

func (r *MonitoringRepository) SaveAuditLog(ctx context.Context, a *models.AuditLog) error {
	return r.db.WithContext(ctx).Create(a).Error
}

func (r *MonitoringRepository) ListAuditLogs(ctx context.Context, orgID uuid.UUID, limit int) ([]models.AuditLog, error) {
	var logs []models.AuditLog
	q := r.db.WithContext(ctx).Order("timestamp desc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&logs).Error
	return logs, err
}

func (r *MonitoringRepository) UpsertMonitoringSession(ctx context.Context, s *models.MonitoringSession) error {
	return r.db.WithContext(ctx).Create(s).Error
}
