package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"examshield/internal/models"
	"examshield/internal/realtime"
	"examshield/internal/repositories"
	"examshield/internal/store"
)

// Recorder is the single path through which violations are written, so
// cooldowns, integrity scoring and live alerts behave identically for
// browser, AI and proctor sources.
type Recorder struct {
	repo *repositories.Repo
	rdb  *store.Redis
	hub  *realtime.Hub
	log  *slog.Logger
}

func NewRecorder(repo *repositories.Repo, rdb *store.Redis, hub *realtime.Hub, log *slog.Logger) *Recorder {
	return &Recorder{repo: repo, rdb: rdb, hub: hub, log: log}
}

type ViolationInput struct {
	SessionID  int64
	ExamID     int64
	EventType  string
	Severity   models.Severity
	Source     models.ViolationSource
	Confidence *float64
	MediaRef   string
	Metadata   map[string]any
	OccurredAt time.Time
	Cooldown   time.Duration
}

// Record stores a violation unless the same event fired for this session
// within its cooldown. It returns (nil, nil) when suppressed.
func (r *Recorder) Record(ctx context.Context, in ViolationInput) (*models.Violation, error) {
	if in.Cooldown > 0 {
		first, err := r.rdb.Once(ctx, fmt.Sprintf("cd:%d:%s", in.SessionID, in.EventType), in.Cooldown)
		if err != nil {
			r.log.Warn("violation cooldown unavailable; recording anyway", "err", err)
		} else if !first {
			return nil, nil
		}
	}
	if in.OccurredAt.IsZero() {
		in.OccurredAt = time.Now().UTC()
	}
	v := &models.Violation{
		SessionID: in.SessionID, EventType: in.EventType, Severity: in.Severity, Source: in.Source,
		OccurredAt: in.OccurredAt, AIConfidence: in.Confidence, Metadata: in.Metadata,
		ReviewStatus: models.ReviewPending,
	}
	if in.MediaRef != "" {
		ref := in.MediaRef
		v.MediaRef = &ref
	}
	var score float64
	err := r.repo.Tx(ctx, func(tx *repositories.Repo) error {
		if err := tx.CreateViolation(ctx, v); err != nil {
			return err
		}
		var err error
		score, err = tx.AdjustIntegrity(ctx, in.SessionID, Penalty(in.Severity, in.Confidence))
		return err
	})
	if err != nil {
		return nil, err
	}
	r.hub.Publish(in.ExamID, "violation", map[string]any{
		"violation": v, "session_id": in.SessionID, "integrity_score": score,
	})
	return v, nil
}
