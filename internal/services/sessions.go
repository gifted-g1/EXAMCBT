package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"time"

	"examshield/internal/ai"
	"examshield/internal/auth"
	"examshield/internal/config"
	"examshield/internal/httpx"
	"examshield/internal/models"
	"examshield/internal/realtime"
	"examshield/internal/repositories"
	"examshield/internal/store"
)

var (
	ErrConsentRequired    = errors.New("consent required")
	ErrNotOnRoster        = errors.New("matric number not on the roster")
	ErrAlreadySubmitted   = errors.New("already submitted")
	ErrDeviceMismatch     = errors.New("session bound to another device")
	ErrBadFingerprint     = errors.New("missing or malformed device fingerprint")
	ErrFaceMismatch       = errors.New("face does not match")
	ErrNoReference        = errors.New("no reference photo to compare against")
	ErrSessionEnded       = errors.New("exam session has ended")
	ErrLockLost           = errors.New("session lock lost")
	ErrTooFrequent        = errors.New("upload too frequent")
	ErrBadMedia           = errors.New("invalid media")
	ErrOutsideExamNetwork = errors.New("client left the exam network")
	ErrFeatureDisabled    = errors.New("this monitoring feature is turned off for the exam")
)

const (
	MaxFrameBytes  = 512 << 10
	MaxPhotoBytes  = 4 << 20
	MaxAnswerBytes = 20000
	verifyFailWarn = 3
)

var clientFPRe = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)

// DeriveFP hashes the browser-supplied device id into the 64-char value
// stored in exam_sessions.device_fp and used as the Redis lock key. It does
// not include the IP, so a DHCP renewal mid-exam is not a "new device".
func DeriveFP(clientFP string) string {
	if !clientFPRe.MatchString(clientFP) {
		return ""
	}
	sum := sha256.Sum256([]byte("es-device-v1|" + clientFP))
	return hex.EncodeToString(sum[:])
}

type StudentService struct {
	repo     *repositories.Repo
	rdb      *store.Redis
	blobs    store.Blobs
	tokens   *auth.Tokens
	matcher  *FaceMatcher
	recorder *Recorder
	hub      *realtime.Hub
	cfg      *config.Config
	log      *slog.Logger
}

func NewStudentService(repo *repositories.Repo, rdb *store.Redis, blobs store.Blobs, tokens *auth.Tokens,
	matcher *FaceMatcher, recorder *Recorder, hub *realtime.Hub, cfg *config.Config, log *slog.Logger) *StudentService {
	return &StudentService{repo: repo, rdb: rdb, blobs: blobs, tokens: tokens, matcher: matcher,
		recorder: recorder, hub: hub, cfg: cfg, log: log}
}

// GateInfo is what a student sees before identifying themselves.
type GateInfo struct {
	ExamID          int64     `json:"exam_id"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	StartsAt        time.Time `json:"starts_at"`
	EndsAt          time.Time `json:"ends_at"`
	DurationMinutes int       `json:"duration_minutes"` // per-student time limit; 0 = whole window
	ServerTime      time.Time `json:"server_time"`
	ConsentVersion  string    `json:"consent_version"`
	FaceThreshold   float64   `json:"face_match_threshold"`
	// Proctoring tells the page what to require: camera, selfie, fullscreen…
	Proctoring models.Proctoring `json:"proctoring"`
}

func (s *StudentService) Gate(e *models.Exam) GateInfo {
	return GateInfo{ExamID: e.ID, Title: e.Title, Description: e.Description, StartsAt: e.StartsAt,
		EndsAt: e.EndsAt, DurationMinutes: e.DurationMinutes, ServerTime: time.Now().UTC(), ConsentVersion: s.cfg.ConsentVersion,
		FaceThreshold: s.matcher.Threshold(), Proctoring: e.Proctoring.Resolve()}
}

type VerifyInput struct {
	MatricNo string
	Selfie   []byte
	IDPhoto  []byte
	Consent  bool
	ClientFP string
	ClientIP string
}

type VerifyResult struct {
	Token     string        `json:"-"`
	ExpiresAt time.Time     `json:"expires_at"`
	Score     *float64      `json:"face_match_score"`
	Assurance string        `json:"assurance"`
	State     *SessionState `json:"state"`
}

// FaceError reports a failed match with its score.
type FaceError struct {
	Score     float64
	Threshold float64
	Reason    error
}

func (e *FaceError) Error() string { return e.Reason.Error() }
func (e *FaceError) Unwrap() error { return e.Reason }

// Verify identifies a student against the roster and their photo, binds
// the session to this device, and returns a session token. It is also the
// rejoin path after a lost cookie: the same student on the same device
// resumes their existing session.
func (s *StudentService) Verify(ctx context.Context, e *models.Exam, in VerifyInput) (*VerifyResult, error) {
	if !in.Consent {
		return nil, ErrConsentRequired
	}
	fp := DeriveFP(in.ClientFP)
	if fp == "" {
		return nil, ErrBadFingerprint
	}
	matric := auth.NormalizeMatric(in.MatricNo)
	if matric == "" {
		return nil, ErrNotOnRoster
	}
	st, err := s.repo.StudentByMatric(ctx, matric)
	if errors.Is(err, repositories.ErrNotFound) {
		return nil, ErrNotOnRoster
	}
	if err != nil {
		return nil, err
	}

	// Existing session checks before taking locks.
	if existing, err := s.findSession(ctx, e.ID, st.ID); err != nil {
		return nil, err
	} else if existing != nil {
		if existing.Status == models.SessionSubmitted || existing.Status == models.SessionClosed {
			return nil, ErrAlreadySubmitted
		}
		// Time limit used up (e.g. the computer was off): submit, don't resume.
		if time.Now().After(e.Deadline(existing.StartedAt).Add(s.cfg.SubmitGrace)) {
			if _, err := s.finalize(ctx, existing.ID, true); err != nil {
				return nil, err
			}
			return nil, ErrAlreadySubmitted
		}
		if existing.DeviceFP != "" && existing.DeviceFP != fp {
			return nil, ErrDeviceMismatch
		}
	}

	if err := s.rdb.ClaimSession(ctx, fp, e.ID, st.ID, s.cfg.SessionLockTTL); err != nil {
		return nil, err
	}
	release := func() { _ = s.rdb.ReleaseSession(context.WithoutCancel(ctx), fp, e.ID, st.ID) }

	// With face verification switched off for this exam, the matric number +
	// consent identify the student; no photo is taken or stored.
	fr := &faceResult{}
	if e.Proctoring.Resolve().FaceVerification {
		fr, err = s.matchFace(ctx, e, st, in)
	}
	if err != nil {
		release()
		s.noteVerifyFailure(ctx, e, st, err)
		return nil, err
	}

	now := time.Now().UTC()
	var sess *models.ExamSession
	err = s.repo.Tx(ctx, func(tx *repositories.Repo) error {
		existing, err := tx.SessionByExamStudentForUpdate(ctx, e.ID, st.ID)
		if err != nil && !errors.Is(err, repositories.ErrNotFound) {
			return err
		}
		fields := map[string]any{
			"device_fp": fp, "client_ip": in.ClientIP, "status": models.SessionActive,
			"face_match_score": fr.score, "reference_key": fr.refKey, "id_photo_key": fr.idKey,
			"consent_at": now, "consent_version": s.cfg.ConsentVersion, "last_seen_at": now,
		}
		if existing != nil {
			if existing.Status == models.SessionSubmitted || existing.Status == models.SessionClosed {
				return ErrAlreadySubmitted
			}
			if existing.DeviceFP != "" && existing.DeviceFP != fp {
				return ErrDeviceMismatch
			}
			if err := tx.UpdateSession(ctx, existing.ID, fields); err != nil {
				return err
			}
			sess, err = tx.SessionByID(ctx, existing.ID)
			return err
		}
		sess = &models.ExamSession{
			ExamID: e.ID, StudentID: st.ID, DeviceFP: fp, ClientIP: in.ClientIP, StartedAt: now,
			LastSeenAt: &now, IntegrityScore: 100, FaceMatchScore: fr.score, ReferenceKey: fr.refKey,
			IDPhotoKey: fr.idKey, ConsentAt: &now, ConsentVersion: s.cfg.ConsentVersion, Status: models.SessionActive,
		}
		return tx.CreateSession(ctx, sess)
	})
	if err != nil {
		release()
		return nil, err
	}

	if fr.idOnly {
		_, _ = s.recorder.Record(ctx, ViolationInput{SessionID: sess.ID, ExamID: e.ID, EventType: "NO_REFERENCE_PHOTO",
			Severity: models.SevMedium, Source: models.SourceAI,
			Metadata: map[string]any{"note": "matched against ID card only; roster photo missing"}})
	}
	if fr.needsReview != "" {
		_, _ = s.recorder.Record(ctx, ViolationInput{SessionID: sess.ID, ExamID: e.ID, EventType: "FACE_REVIEW_REQUIRED",
			Severity: models.SevHigh, Source: models.SourceAI,
			Metadata: map[string]any{"reason": fr.needsReview, "assurance": s.matcher.Assurance()}})
	}

	expires := e.Deadline(sess.StartedAt).Add(s.cfg.SubmitGrace)
	token, err := s.tokens.IssueStudent(sess.ID, e.ID, st.ID, fp, expires)
	if err != nil {
		release()
		return nil, err
	}
	s.hub.Publish(e.ID, "session", map[string]any{"session_id": sess.ID, "event": "joined",
		"matric_no": st.MatricNo, "full_name": st.FullName, "face_match_score": fr.score, "needs_review": fr.needsReview != ""})

	state, err := s.State(ctx, sess.ID)
	if err != nil {
		return nil, err
	}
	return &VerifyResult{Token: token, ExpiresAt: expires, Score: fr.score, Assurance: s.matcher.Assurance(), State: state}, nil
}

func (s *StudentService) findSession(ctx context.Context, examID, studentID int64) (*models.ExamSession, error) {
	sess, err := s.repo.SessionByExamStudent(ctx, examID, studentID)
	if errors.Is(err, repositories.ErrNotFound) {
		return nil, nil
	}
	return sess, err
}

// faceResult is the outcome of identity matching at entry.
type faceResult struct {
	score       *float64 // nil when there was nothing to compare against
	refKey      string
	idKey       string
	idOnly      bool   // compared with the ID card because no roster photo exists
	needsReview string // non-empty: admitted, but a proctor must check identity
}

// matchFace scores the live selfie against the roster photo (preferred) or
// the face on the ID card, and stores both captures as evidence. The selfie
// itself must always show exactly one face. Whether a low score or a missing
// reference blocks the student depends on FaceMatcher.Enforced().
func (s *StudentService) matchFace(ctx context.Context, e *models.Exam, st *models.Student, in VerifyInput) (*faceResult, error) {
	if len(in.Selfie) == 0 || len(in.Selfie) > MaxPhotoBytes || len(in.IDPhoto) > MaxPhotoBytes {
		return nil, ErrBadMedia
	}
	selfie, err := ai.Decode(in.Selfie)
	if err != nil {
		return nil, ErrBadMedia
	}
	probe, err := s.matcher.Embedding(selfie)
	if err != nil {
		return nil, err
	}
	res := &faceResult{}
	enforced := s.matcher.Enforced()

	var ref []float32
	if key := RosterPhotoKey(st); key != "" {
		if data, err := s.blobs.Get(ctx, key); err == nil {
			if img, err := ai.Decode(data); err == nil {
				ref, _ = s.matcher.Embedding(img)
			}
		}
	}
	if ref == nil && len(in.IDPhoto) > 0 {
		if idImg, err := ai.Decode(in.IDPhoto); err == nil {
			// ID cards may carry a hologram or second small face; take the largest.
			if face, _, err := s.matcher.SingleFace(idImg); err == nil {
				if ref, err = s.matcher.engine.Embed(idImg, *face); err != nil {
					return nil, err
				}
				res.idOnly = true
			}
		}
	}

	switch {
	case ref == nil && enforced:
		return nil, &FaceError{Reason: ErrNoReference, Threshold: s.matcher.Threshold()}
	case ref == nil:
		res.needsReview = "no roster photo on file and no face found on the ID photo"
	default:
		score := s.matcher.Score(probe, ref)
		res.score = &score
		if !s.matcher.Matches(score) {
			if enforced {
				return nil, &FaceError{Score: score, Threshold: s.matcher.Threshold(), Reason: ErrFaceMismatch}
			}
			res.needsReview = fmt.Sprintf("match score %.3f below %.2f (low-assurance matcher)", score, s.matcher.Threshold())
		}
	}

	ts := time.Now().UTC().UnixMilli()
	res.refKey = fmt.Sprintf("verify/%d/%d/selfie-%d.jpg", e.ID, st.ID, ts)
	if err := s.blobs.Put(ctx, res.refKey, in.Selfie, "image/jpeg"); err != nil {
		return nil, err
	}
	if len(in.IDPhoto) > 0 {
		res.idKey = fmt.Sprintf("verify/%d/%d/id-%d.jpg", e.ID, st.ID, ts)
		if err := s.blobs.Put(ctx, res.idKey, in.IDPhoto, "image/jpeg"); err != nil {
			return nil, err
		}
	}
	return res, nil
}

func (s *StudentService) noteVerifyFailure(ctx context.Context, e *models.Exam, st *models.Student, err error) {
	var fe *FaceError
	if !errors.As(err, &fe) && !errors.Is(err, ErrNoFace) && !errors.Is(err, ErrMultipleFaces) {
		return
	}
	key := fmt.Sprintf("verify_fail:%d:%d", e.ID, st.ID)
	n, _ := s.rdb.Incr(ctx, key).Result()
	s.rdb.Expire(ctx, key, time.Hour)
	if n >= verifyFailWarn {
		s.hub.Publish(e.ID, "verify_failed", map[string]any{"matric_no": st.MatricNo, "full_name": st.FullName,
			"attempts": n, "reason": err.Error()})
	}
}

// RosterPhotoKey returns the blob key of a student's roster photo, if the
// photo was uploaded through Exam Shield (photo_url = "blob:<key>").
func RosterPhotoKey(st *models.Student) string {
	if len(st.PhotoURL) > 5 && st.PhotoURL[:5] == "blob:" {
		return st.PhotoURL[5:]
	}
	return ""
}

// --- In-exam operations -------------------------------------------------------------

type QuestionView struct {
	ID       int64               `json:"id"`
	Position int                 `json:"position"`
	Kind     models.QuestionKind `json:"kind"`
	Prompt   string              `json:"prompt"`
	Options  []string            `json:"options,omitempty"`
	Marks    float64             `json:"marks"`
	Blanks   int                 `json:"blanks,omitempty"` // GAP: number of blanks (never the answers)
}

// SessionState never includes correct answers.
type SessionState struct {
	SessionID       int64                `json:"session_id"`
	Status          models.SessionStatus `json:"status"`
	ExamID          int64                `json:"exam_id"`
	ExamTitle       string               `json:"exam_title"`
	EndsAt          time.Time            `json:"ends_at"` // this student's deadline
	DurationMinutes int                  `json:"duration_minutes"`
	ServerTime      time.Time            `json:"server_time"`
	Student         struct {
		MatricNo string `json:"matric_no"`
		FullName string `json:"full_name"`
	} `json:"student"`
	Questions  []QuestionView    `json:"questions"`
	Answers    map[string]string `json:"answers"`
	Proctoring models.Proctoring `json:"proctoring"`
}

func (s *StudentService) State(ctx context.Context, sessionID int64) (*SessionState, error) {
	sess, err := s.repo.SessionByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	e, err := s.repo.ExamByID(ctx, sess.ExamID)
	if err != nil {
		return nil, err
	}
	qs, err := s.repo.Questions(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	ans, err := s.repo.Answers(ctx, sess.ID)
	if err != nil {
		return nil, err
	}
	st := &SessionState{SessionID: sess.ID, Status: sess.Status, ExamID: e.ID, ExamTitle: e.Title, Proctoring: e.Proctoring.Resolve(),
		EndsAt: e.Deadline(sess.StartedAt), DurationMinutes: e.DurationMinutes, ServerTime: time.Now().UTC(), Answers: map[string]string{}}
	if sess.Student != nil {
		st.Student.MatricNo, st.Student.FullName = sess.Student.MatricNo, sess.Student.FullName
	}
	st.Questions = make([]QuestionView, len(qs))
	for i, q := range qs {
		st.Questions[i] = QuestionView{ID: q.ID, Position: q.Position, Kind: q.Kind, Prompt: q.Prompt, Options: q.Options, Marks: q.Marks, Blanks: len(q.Blanks)}
	}
	for _, a := range ans {
		st.Answers[strconv.FormatInt(a.QuestionID, 10)] = a.Response
	}
	return st, nil
}

// activeSession loads the session and exam and checks the session may still
// change: status ACTIVE and the exam window (plus grace) not over.
func (s *StudentService) activeSession(ctx context.Context, c *auth.StudentClaims) (*models.ExamSession, *models.Exam, error) {
	sess, err := s.repo.SessionByID(ctx, c.SessionID())
	if err != nil {
		return nil, nil, err
	}
	if sess.ExamID != c.ExamID || sess.StudentID != c.StudentID || sess.DeviceFP != c.DeviceFP {
		return nil, nil, ErrDeviceMismatch
	}
	if sess.Status != models.SessionActive {
		return nil, nil, ErrSessionEnded
	}
	e, err := s.repo.ExamByID(ctx, sess.ExamID)
	if err != nil {
		return nil, nil, err
	}
	if e.Status == models.ExamClosed || time.Now().After(e.Deadline(sess.StartedAt).Add(s.cfg.SubmitGrace)) {
		return nil, nil, ErrSessionEnded
	}
	if ip := httpx.ClientIPFrom(ctx); ip != nil && !s.cfg.OpenAccess() && !(ip.IsLoopback() && s.cfg.AllowLoopbackExam) {
		if n, err := config.ParseCIDR(e.LANCIDR); err != nil || !n.Contains(ip) {
			return nil, nil, ErrOutsideExamNetwork
		}
	}
	return sess, e, nil
}

func (s *StudentService) SaveAnswer(ctx context.Context, c *auth.StudentClaims, questionID int64, response string) (time.Time, error) {
	sess, e, err := s.activeSession(ctx, c)
	if err != nil {
		return time.Time{}, err
	}
	if len(response) > MaxAnswerBytes {
		return time.Time{}, invalid("answer too long")
	}
	qs, err := s.repo.Questions(ctx, e.ID)
	if err != nil {
		return time.Time{}, err
	}
	var q *models.Question
	for i := range qs {
		if qs[i].ID == questionID {
			q = &qs[i]
		}
	}
	if q == nil {
		return time.Time{}, invalid("question does not belong to this exam")
	}
	if q.Kind == models.QuestionMCQ && response != "" {
		idx, err := strconv.Atoi(response)
		if err != nil || idx < 0 || idx >= len(q.Options) {
			return time.Time{}, invalid("response must be an option index")
		}
	}
	if q.Kind == models.QuestionGap && response != "" {
		if g := GapAnswers(response); g == nil || len(g) > len(q.Blanks) {
			return time.Time{}, invalid("response must be a JSON list with one answer per blank")
		}
	}
	now := time.Now().UTC()
	if err := s.repo.UpsertAnswer(ctx, &models.Answer{SessionID: sess.ID, QuestionID: q.ID, Response: response, SavedAt: now}); err != nil {
		return time.Time{}, err
	}
	return now, nil
}

// Heartbeat keeps the device/student locks alive. If the locks expired (a
// long network drop) but nobody else claimed them, they are re-acquired.
// The returned exam carries the current end time and proctoring switches, so
// time extensions and toggles reach students within one heartbeat.
func (s *StudentService) Heartbeat(ctx context.Context, c *auth.StudentClaims) (*models.Exam, time.Time, error) {
	sess, e, err := s.activeSession(ctx, c)
	if err != nil {
		return nil, time.Time{}, err
	}
	ok, err := s.rdb.RefreshSession(ctx, c.DeviceFP, e.ID, sess.StudentID, s.cfg.SessionLockTTL)
	if err != nil {
		return nil, time.Time{}, err
	}
	if !ok {
		if err := s.rdb.ClaimSession(ctx, c.DeviceFP, e.ID, sess.StudentID, s.cfg.SessionLockTTL); err != nil {
			return nil, time.Time{}, ErrLockLost
		}
	}
	now := time.Now().UTC()
	if err := s.repo.TouchSession(ctx, sess.ID, now); err != nil {
		return nil, time.Time{}, err
	}
	return e, e.Deadline(sess.StartedAt), nil
}

// eventEnabled reports whether the exam's proctoring settings record a given
// browser event. Disabled events are accepted and dropped, so an older or
// cached client can't fill the alert feed with checks the examiner turned off.
func eventEnabled(p models.Proctoring, eventType string) bool {
	switch eventType {
	case "FULLSCREEN_EXIT":
		return p.FullscreenRequired
	case "TAB_SWITCH", "WINDOW_BLUR", "MULTIPLE_DISPLAYS":
		return p.TabSwitchDetection
	case "COPY_PASTE", "CONTEXT_MENU", "DEVTOOLS_SHORTCUT", "PRINT_SCREEN":
		return p.ClipboardBlocking
	case "CAMERA_UNAVAILABLE":
		return p.CameraMonitoring
	case "MIC_UNAVAILABLE":
		return p.AudioMonitoring
	}
	return true
}

func (s *StudentService) ReportEvent(ctx context.Context, c *auth.StudentClaims, eventType string, meta map[string]any) error {
	spec, ok := ClientEvents[eventType]
	if !ok {
		return invalid("unknown event type")
	}
	sess, e, err := s.activeSession(ctx, c)
	if err != nil {
		return err
	}
	if !eventEnabled(e.Proctoring.Resolve(), eventType) {
		return nil
	}
	// Keep only small scalar metadata from the browser.
	clean := map[string]any{}
	for k, v := range meta {
		if len(clean) >= 8 || len(k) > 32 {
			break
		}
		switch t := v.(type) {
		case string:
			if len(t) <= 200 {
				clean[k] = t
			}
		case float64, bool:
			clean[k] = t
		}
	}
	_, err = s.recorder.Record(ctx, ViolationInput{SessionID: sess.ID, ExamID: e.ID, EventType: eventType,
		Severity: spec.Severity, Source: models.SourceClient, Metadata: clean, Cooldown: spec.Cooldown})
	return err
}

// IngestMedia stores a webcam frame or audio clip and queues it for AI.
func (s *StudentService) IngestMedia(ctx context.Context, c *auth.StudentClaims, kind string, data []byte) error {
	sess, e, err := s.activeSession(ctx, c)
	if err != nil {
		return err
	}
	p := e.Proctoring.Resolve()
	if (kind == "frame" && !p.CameraMonitoring) || (kind == "audio" && !p.AudioMonitoring) {
		return ErrFeatureDisabled
	}
	var ext, ctype string
	switch kind {
	case "frame":
		if len(data) == 0 || len(data) > MaxFrameBytes {
			return ErrBadMedia
		}
		if _, err := ai.Decode(data); err != nil {
			return ErrBadMedia
		}
		ext, ctype = "jpg", "image/jpeg"
	case "audio":
		if _, err := DecodePCM16(data); err != nil {
			return ErrBadMedia
		}
		ext, ctype = "pcm", "application/octet-stream"
	default:
		return ErrBadMedia
	}
	first, err := s.rdb.Once(ctx, fmt.Sprintf("media:%s:%d", kind, sess.ID), s.cfg.FrameMinInterval)
	if err == nil && !first {
		return ErrTooFrequent
	}
	now := time.Now().UTC().UnixMilli()
	key := fmt.Sprintf("%ss/%d/%d/%d.%s", kind, e.ID, sess.ID, now, ext)
	if err := s.blobs.Put(ctx, key, data, ctype); err != nil {
		return err
	}
	if kind == "frame" {
		s.rdb.Set(ctx, LatestFrameKey(sess.ID), key, 10*time.Minute)
	}
	return s.rdb.Publish(ctx, store.StreamFrames, store.FrameJob{Kind: kind, ExamID: e.ID, SessionID: sess.ID,
		Key: key, ReferenceKey: sess.ReferenceKey, CapturedAt: now})
}

func LatestFrameKey(sessionID int64) string { return fmt.Sprintf("latest_frame:%d", sessionID) }

// Submit finalises the student's own session.
func (s *StudentService) Submit(ctx context.Context, c *auth.StudentClaims) (*models.ExamSession, error) {
	sess, err := s.repo.SessionByID(ctx, c.SessionID())
	if err != nil {
		return nil, err
	}
	if sess.DeviceFP != c.DeviceFP {
		return nil, ErrDeviceMismatch
	}
	return s.finalize(ctx, sess.ID, false)
}

// finalize grades and closes a session idempotently and releases its locks.
func (s *StudentService) finalize(ctx context.Context, sessionID int64, auto bool) (*models.ExamSession, error) {
	var out *models.ExamSession
	var changed bool
	err := s.repo.Tx(ctx, func(tx *repositories.Repo) error {
		sess, err := tx.SessionByIDForUpdate(ctx, sessionID)
		if err != nil {
			return err
		}
		if sess.Status == models.SessionSubmitted || sess.Status == models.SessionClosed {
			out = sess
			return nil
		}
		score, err := grade(ctx, tx, sess)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if err := tx.UpdateSession(ctx, sess.ID, map[string]any{"status": models.SessionSubmitted,
			"ended_at": now, "score": score, "auto_submitted": auto}); err != nil {
			return err
		}
		sess.Status, sess.EndedAt, sess.Score, sess.AutoSubmitted = models.SessionSubmitted, &now, &score, auto
		out, changed = sess, true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if changed {
		bg := context.WithoutCancel(ctx)
		if out.DeviceFP != "" {
			_ = s.rdb.ReleaseSession(bg, out.DeviceFP, out.ExamID, out.StudentID)
		}
		s.hub.Publish(out.ExamID, "session", map[string]any{"session_id": out.ID, "event": "submitted", "auto": auto})
	}
	return out, nil
}

func grade(ctx context.Context, tx *repositories.Repo, sess *models.ExamSession) (float64, error) {
	qs, err := tx.Questions(ctx, sess.ExamID)
	if err != nil {
		return 0, err
	}
	ans, err := tx.Answers(ctx, sess.ID)
	if err != nil {
		return 0, err
	}
	byQ := answersByQuestion(ans)
	var score float64
	for _, q := range qs {
		if _, awarded := markAnswer(q, byQ[q.ID]); awarded != nil {
			score += *awarded
		}
	}
	return score, nil
}

// AutoSubmitOverdue finalises open sessions whose personal time limit (plus
// grace) has passed while the exam itself is still open.
func (s *StudentService) AutoSubmitOverdue(ctx context.Context) {
	exams, err := s.repo.ActiveTimedExams(ctx)
	if err != nil {
		s.log.Error("auto-submit overdue: list exams", "err", err)
		return
	}
	now := time.Now()
	for i := range exams {
		e := &exams[i]
		sessions, err := s.repo.OpenSessionsForExam(ctx, e.ID)
		if err != nil {
			s.log.Error("auto-submit overdue: list sessions", "exam_id", e.ID, "err", err)
			continue
		}
		for _, sess := range sessions {
			if now.After(e.Deadline(sess.StartedAt).Add(s.cfg.SubmitGrace)) {
				if _, err := s.finalize(ctx, sess.ID, true); err != nil {
					s.log.Error("auto-submit overdue: finalize", "session_id", sess.ID, "err", err)
				}
			}
		}
	}
}

// AutoSubmitExam finalises every open session of a closed exam.
func (s *StudentService) AutoSubmitExam(ctx context.Context, examID int64) {
	sessions, err := s.repo.OpenSessionsForExam(ctx, examID)
	if err != nil {
		s.log.Error("auto-submit: list sessions", "exam_id", examID, "err", err)
		return
	}
	for _, sess := range sessions {
		if _, err := s.finalize(ctx, sess.ID, true); err != nil {
			s.log.Error("auto-submit: finalize", "session_id", sess.ID, "err", err)
		}
	}
}

// Terminate is a proctor action: end a session immediately.
func (s *StudentService) Terminate(ctx context.Context, sessionID int64) (*models.ExamSession, error) {
	return s.finalize(ctx, sessionID, true)
}

// Unlock clears a session's device binding so the student can continue on a
// different machine (after re-verifying their face).
func (s *StudentService) Unlock(ctx context.Context, sess *models.ExamSession) error {
	if err := s.rdb.ForceReleaseStudent(ctx, sess.ExamID, sess.StudentID); err != nil {
		return err
	}
	if sess.DeviceFP != "" {
		_ = s.rdb.Del(ctx, store.DeviceLockKey(sess.DeviceFP)).Err()
	}
	return s.repo.UpdateSession(ctx, sess.ID, map[string]any{"device_fp": ""})
}
