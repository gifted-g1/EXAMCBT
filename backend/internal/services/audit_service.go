package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"examshield/internal/models"
	"examshield/internal/repositories"
)

// AuditService is the single write path for AuditLog rows, ensuring
// every significant action across the platform is recorded uniformly.
type AuditService struct {
	repo   *repositories.MonitoringRepository
	logger *slog.Logger
}

func NewAuditService(repo *repositories.MonitoringRepository, logger *slog.Logger) *AuditService {
	return &AuditService{repo: repo, logger: logger}
}

func (s *AuditService) Log(ctx context.Context, actorID uuid.UUID, role models.Role, action, resource, resourceID, ip, metadata string) {
	entry := &models.AuditLog{
		ActorID:    actorID,
		ActorRole:  role,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		IPAddress:  ip,
		Metadata:   metadata,
		Timestamp:  time.Now(),
	}
	if err := s.repo.SaveAuditLog(ctx, entry); err != nil {
		// Audit logging must never crash the request path; log locally
		// so the failure is at least observable in server logs.
		s.logger.Error("failed to persist audit log", "action", action, "error", err)
	}
}
