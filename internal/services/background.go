package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/datatypes"

	"examshield/internal/models"
	"examshield/internal/realtime"
	"examshield/internal/repositories"
	"examshield/internal/store"
)

// FindingsConsumer persists AI results from the "findings" stream and turns
// flags into violations. Messages are ACKed only after they are stored, so a
// crash redelivers them (at-least-once); the per-event cooldown absorbs the
// resulting duplicates.
type FindingsConsumer struct {
	repo     *repositories.Repo
	rdb      *store.Redis
	recorder *Recorder
	hub      *realtime.Hub
	consumer string
	log      *slog.Logger
}

func NewFindingsConsumer(repo *repositories.Repo, rdb *store.Redis, recorder *Recorder, hub *realtime.Hub, consumer string, log *slog.Logger) *FindingsConsumer {
	return &FindingsConsumer{repo: repo, rdb: rdb, recorder: recorder, hub: hub, consumer: consumer, log: log}
}

func (f *FindingsConsumer) Run(ctx context.Context) error {
	if err := f.rdb.EnsureGroup(ctx, store.StreamFindings, store.GroupAPI); err != nil {
		return err
	}
	pending := true // first drain anything this consumer left un-ACKed
	lastClaim := time.Time{}
	for ctx.Err() == nil {
		if time.Since(lastClaim) > store.StaleClaimInterval {
			lastClaim = time.Now()
			stale, err := f.rdb.ClaimStale(ctx, store.StreamFindings, store.GroupAPI, f.consumer, store.StaleMinIdle, 64)
			if err != nil {
				f.log.Error("findings: claim stale", "err", err)
			}
			for _, m := range stale {
				f.process(ctx, m)
			}
		}
		msgs, err := f.rdb.ReadGroup(ctx, store.StreamFindings, store.GroupAPI, f.consumer, 32, 2*time.Second, pending)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			f.log.Error("findings: read", "err", err)
			sleep(ctx, 2*time.Second)
			continue
		}
		if pending && len(msgs) == 0 {
			pending = false
			continue
		}
		for _, m := range msgs {
			f.process(ctx, m)
		}
	}
	return nil
}

// process handles one message and ACKs it only once it is stored.
func (f *FindingsConsumer) process(ctx context.Context, m store.Message) {
	if err := f.handle(ctx, m.Data); err != nil {
		f.log.Error("findings: handle (will retry)", "id", m.ID, "err", err)
		return
	}
	if err := f.rdb.Ack(ctx, store.StreamFindings, store.GroupAPI, m.ID); err != nil {
		f.log.Error("findings: ack", "err", err)
	}
}

func (f *FindingsConsumer) handle(ctx context.Context, raw []byte) error {
	var msg store.FindingsMsg
	if err := json.Unmarshal(raw, &msg); err != nil {
		f.log.Error("findings: dropping malformed message", "err", err)
		return nil // poison message: ACK and move on
	}
	sess, err := f.repo.SessionByID(ctx, msg.SessionID)
	if errors.Is(err, repositories.ErrNotFound) {
		return nil // session erased meanwhile
	}
	if err != nil {
		return err
	}
	at := time.UnixMilli(msg.CapturedAt).UTC()
	rows := make([]models.AIFinding, 0, len(msg.Inferences))
	for name, inf := range msg.Inferences {
		rows = append(rows, models.AIFinding{SessionID: sess.ID, ModelName: truncate(name, 50),
			Inference: datatypes.JSON(inf), CreatedAt: at})
	}
	if err := f.repo.CreateFindings(ctx, rows); err != nil {
		return err
	}
	if sess.Status != models.SessionActive {
		return nil // analysis of frames captured just before submit: keep findings, no new alerts
	}
	for _, fl := range msg.Flags {
		spec, ok := AIEvents[fl.EventType]
		if !ok {
			continue
		}
		conf := fl.Confidence
		if _, err := f.recorder.Record(ctx, ViolationInput{
			SessionID: sess.ID, ExamID: sess.ExamID, EventType: fl.EventType, Severity: spec.Severity,
			Source: models.SourceAI, Confidence: &conf, MediaRef: msg.MediaKey, Metadata: fl.Metadata,
			OccurredAt: at, Cooldown: spec.Cooldown,
		}); err != nil {
			return err
		}
	}
	return nil
}

// Scheduler opens SCHEDULED exams at starts_at, closes exams after ends_at
// (+ submit grace, auto-submitting open sessions) and purges media past the
// retention period. With several API replicas only the Redis lock holder
// runs each tick.
type Scheduler struct {
	repo      *repositories.Repo
	rdb       *store.Redis
	blobs     store.Blobs
	exams     *ExamService
	grace     time.Duration
	retention time.Duration
	log       *slog.Logger
}

func NewScheduler(repo *repositories.Repo, rdb *store.Redis, blobs store.Blobs, exams *ExamService, grace time.Duration, retentionDays int, log *slog.Logger) *Scheduler {
	return &Scheduler{repo: repo, rdb: rdb, blobs: blobs, exams: exams, grace: grace,
		retention: time.Duration(retentionDays) * 24 * time.Hour, log: log}
}

func (s *Scheduler) Run(ctx context.Context) {
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	lastJanitor := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		if ok, err := s.rdb.Once(ctx, "lock:scheduler", 9*time.Second); err != nil || !ok {
			continue
		}
		s.lifecycle(ctx)
		if time.Since(lastJanitor) > time.Hour {
			lastJanitor = time.Now()
			s.janitor(ctx)
		}
	}
}

func (s *Scheduler) lifecycle(ctx context.Context) {
	now := time.Now().UTC()
	toOpen, err := s.repo.ExamsToActivate(ctx, now)
	if err != nil {
		s.log.Error("scheduler: list exams to open", "err", err)
	}
	for _, e := range toOpen {
		if _, err := s.exams.Transition(ctx, e.ID, "open"); err != nil {
			s.log.Warn("scheduler: could not auto-open exam", "exam_id", e.ID, "err", err)
		} else {
			s.log.Info("scheduler: exam opened", "exam_id", e.ID)
		}
	}
	toClose, err := s.repo.ExamsToClose(ctx, now.Add(-s.grace))
	if err != nil {
		s.log.Error("scheduler: list exams to close", "err", err)
	}
	for _, e := range toClose {
		if _, err := s.exams.Transition(ctx, e.ID, "close"); err != nil {
			s.log.Error("scheduler: could not auto-close exam", "exam_id", e.ID, "err", err)
		} else {
			s.log.Info("scheduler: exam closed", "exam_id", e.ID)
		}
	}
}

func (s *Scheduler) janitor(ctx context.Context) {
	if s.retention > 0 {
		exams, err := s.repo.ExamsForMediaPurge(ctx, time.Now().UTC().Add(-s.retention))
		if err != nil {
			s.log.Error("janitor: list exams", "err", err)
		}
		for _, e := range exams {
			if err := s.PurgeExamMedia(ctx, e.ID); err != nil {
				s.log.Error("janitor: purge media", "exam_id", e.ID, "err", err)
				continue
			}
			s.log.Info("janitor: media purged after retention period", "exam_id", e.ID)
		}
	}
	if n, err := s.repo.PurgeExpiredRefresh(ctx, time.Now().UTC()); err == nil && n > 0 {
		s.log.Info("janitor: expired refresh tokens deleted", "count", n)
	}
}

// PurgeExamMedia deletes every stored image/audio clip for an exam and the
// AI findings derived from them; violations remain as the audit record.
func (s *Scheduler) PurgeExamMedia(ctx context.Context, examID int64) error {
	for _, p := range []string{"frames", "audios", "verify"} {
		if err := s.blobs.DeletePrefix(ctx, fmt.Sprintf("%s/%d/", p, examID)); err != nil {
			return err
		}
	}
	return s.repo.PurgeExamMedia(ctx, examID)
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
