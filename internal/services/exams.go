package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"examshield/internal/auth"
	"examshield/internal/config"
	"examshield/internal/models"
	"examshield/internal/netutil"
	"examshield/internal/realtime"
	"examshield/internal/repositories"
	"examshield/internal/store"
)

var (
	ErrForbidden       = errors.New("forbidden")
	ErrInvalidState    = errors.New("invalid state for this action")
	ErrSubnetConflict  = errors.New("another exam is open on an overlapping subnet; both exams need a PIN")
	ErrPinInUse        = errors.New("PIN already used by another live exam")
	ErrNoQuestions     = errors.New("exam has no questions")
	ErrWindowPassed    = errors.New("exam window has already ended")
	ErrCIDROutsideLAN  = errors.New("lan_cidr does not overlap the server's LAN")
	errValidationGroup = errors.New("validation")
)

// ValidationError carries a user-facing message.
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }
func (e *ValidationError) Unwrap() error { return errValidationGroup }

func invalid(format string, a ...any) error { return &ValidationError{Msg: fmt.Sprintf(format, a...)} }

const openExamsCacheKey = "cache:open_exams"

type ExamService struct {
	repo     *repositories.Repo
	rdb      *store.Redis
	hub      *realtime.Hub
	cfg      *config.Config
	lanNets  []*net.IPNet
	log      *slog.Logger
	OnClosed func(ctx context.Context, examID int64) // auto-submits open sessions
}

func NewExamService(repo *repositories.Repo, rdb *store.Redis, hub *realtime.Hub, cfg *config.Config, lanNets []*net.IPNet, log *slog.Logger) *ExamService {
	return &ExamService{repo: repo, rdb: rdb, hub: hub, cfg: cfg, lanNets: lanNets, log: log}
}

// OpenExams implements middleware.OpenExamSource with a 5 s Redis cache, so
// a hall of students hammering the gate costs one MySQL query per 5 s.
func (s *ExamService) OpenExams(ctx context.Context) ([]models.Exam, error) {
	if raw, err := s.rdb.Get(ctx, openExamsCacheKey).Bytes(); err == nil {
		var exams []models.Exam
		if json.Unmarshal(raw, &exams) == nil {
			return exams, nil
		}
	}
	exams, err := s.repo.OpenExams(ctx, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	// The cached copy must keep pin_code (json omits it only when nil).
	if b, err := json.Marshal(exams); err == nil {
		s.rdb.Set(ctx, openExamsCacheKey, b, 5*time.Second)
	}
	return exams, nil
}

func (s *ExamService) invalidate(ctx context.Context) { s.rdb.Del(ctx, openExamsCacheKey) }

// CanManage: admins manage every exam; proctors manage exams they created.
func CanManage(actorID int64, isAdmin bool, e *models.Exam) bool {
	return isAdmin || e.CreatedBy == actorID
}

type ExamInput struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	LANCIDR     string    `json:"lan_cidr"`
	RequirePIN  bool      `json:"require_pin"`
	// DurationMinutes is each student's time limit (0 = until ends_at).
	DurationMinutes int `json:"duration_minutes"`
	// QuestionsPerStudent draws that many random questions per student
	// from the exam's pool (0 = everyone gets every question).
	QuestionsPerStudent int `json:"questions_per_student"`
	// Proctoring is optional on create; omitted switches default to on.
	Proctoring *models.ProctoringSettings `json:"proctoring,omitempty"`
}

func (s *ExamService) validateInput(in *ExamInput) error {
	in.Title = strings.TrimSpace(in.Title)
	if l := len([]rune(in.Title)); l < 3 || l > 150 {
		return invalid("title must be 3–150 characters")
	}
	if len(in.Description) > 5000 {
		return invalid("description too long")
	}
	if in.StartsAt.IsZero() || in.EndsAt.IsZero() || !in.EndsAt.After(in.StartsAt) {
		return invalid("ends_at must be after starts_at")
	}
	if in.EndsAt.Sub(in.StartsAt) > 12*time.Hour {
		return invalid("exam window cannot exceed 12 hours")
	}
	if in.QuestionsPerStudent < 0 || in.QuestionsPerStudent > 500 {
		return invalid("questions per student must be between 0 and 500 (0 = all questions)")
	}
	if in.DurationMinutes < 0 || in.DurationMinutes > 720 {
		return invalid("time limit must be between 0 and 720 minutes (0 = no limit)")
	}
	if w := in.EndsAt.Sub(in.StartsAt); in.DurationMinutes > 0 && time.Duration(in.DurationMinutes)*time.Minute > w {
		return invalid("the %d-minute time limit is longer than the exam window (%d minutes)", in.DurationMinutes, int(w.Minutes()))
	}
	in.StartsAt, in.EndsAt = in.StartsAt.UTC(), in.EndsAt.UTC()
	if in.LANCIDR == "" {
		in.LANCIDR = s.defaultCIDR()
	}
	n, err := config.ParseCIDR(in.LANCIDR)
	if err != nil {
		return invalid("lan_cidr is not a valid CIDR")
	}
	if s.cfg.OpenAccess() {
		// No hall network in open (cloud/demo) mode: any address may sit the exam.
		in.LANCIDR = n.String()
		return nil
	}
	ones, bits := n.Mask.Size()
	if (bits == 32 && ones < 8) || (bits == 128 && ones < 32) {
		return invalid("lan_cidr is too broad")
	}
	if !n.IP.IsPrivate() && !n.IP.IsLoopback() {
		return invalid("lan_cidr must be a private (RFC 1918 / ULA) network")
	}
	if len(s.lanNets) > 0 && !overlapsAny(n, s.lanNets) {
		return ErrCIDROutsideLAN
	}
	in.LANCIDR = n.String()
	return nil
}

func (s *ExamService) defaultCIDR() string {
	if s.cfg.OpenAccess() {
		return "0.0.0.0/0"
	}
	if len(s.lanNets) > 0 {
		return s.lanNets[0].String()
	}
	return "192.168.0.0/16"
}

func overlaps(a, b *net.IPNet) bool { return a.Contains(b.IP) || b.Contains(a.IP) }

func overlapsAny(n *net.IPNet, nets []*net.IPNet) bool {
	for _, x := range nets {
		if overlaps(n, x) {
			return true
		}
	}
	return false
}

func (s *ExamService) Create(ctx context.Context, actorID int64, in ExamInput) (*models.Exam, error) {
	if err := s.validateInput(&in); err != nil {
		return nil, err
	}
	e := &models.Exam{Title: in.Title, Description: in.Description, CreatedBy: actorID,
		StartsAt: in.StartsAt, EndsAt: in.EndsAt, LANCIDR: in.LANCIDR, Status: models.ExamDraft, DurationMinutes: in.DurationMinutes, QuestionsPerStudent: in.QuestionsPerStudent}
	if in.Proctoring != nil {
		e.Proctoring = in.Proctoring.Resolve().Settings()
	}
	if in.RequirePIN {
		pin, err := s.uniquePIN(ctx, 0)
		if err != nil {
			return nil, err
		}
		e.PinCode = &pin
	}
	if err := s.repo.CreateExam(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

// Update edits an exam. Window and subnet are frozen once ACTIVE/CLOSED,
// except that an ACTIVE exam's end may be extended.
func (s *ExamService) Update(ctx context.Context, e *models.Exam, in ExamInput) (*models.Exam, error) {
	if e.Status == models.ExamClosed {
		return nil, ErrInvalidState
	}
	if e.Status == models.ExamActive {
		if !in.StartsAt.Equal(e.StartsAt) || in.LANCIDR != e.LANCIDR {
			return nil, invalid("only title, description and end time can change while the exam is active")
		}
		if in.EndsAt.Before(time.Now()) {
			return nil, invalid("ends_at must be in the future for an active exam")
		}
	}
	if err := s.validateInput(&in); err != nil {
		return nil, err
	}
	fields := map[string]any{"title": in.Title, "description": in.Description,
		"starts_at": in.StartsAt, "ends_at": in.EndsAt, "lan_cidr": in.LANCIDR, "duration_minutes": in.DurationMinutes, "questions_per_student": in.QuestionsPerStudent}
	if err := s.repo.UpdateExam(ctx, e.ID, fields); err != nil {
		return nil, err
	}
	s.invalidate(ctx)
	return s.repo.ExamByID(ctx, e.ID)
}

// Transition applies a lifecycle action: schedule | open | close.
func (s *ExamService) Transition(ctx context.Context, examID int64, action string) (*models.Exam, error) {
	var closed bool
	err := s.repo.Tx(ctx, func(tx *repositories.Repo) error {
		e, err := tx.ExamByIDForUpdate(ctx, examID)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		switch action {
		case "schedule":
			if e.Status != models.ExamDraft {
				return ErrInvalidState
			}
			if !e.EndsAt.After(now) {
				return ErrWindowPassed
			}
			if err := requireQuestions(ctx, tx, e.ID); err != nil {
				return err
			}
			return tx.UpdateExam(ctx, e.ID, map[string]any{"status": models.ExamScheduled})
		case "open":
			if e.Status != models.ExamDraft && e.Status != models.ExamScheduled {
				return ErrInvalidState
			}
			if !e.EndsAt.After(now) {
				return ErrWindowPassed
			}
			if err := requireQuestions(ctx, tx, e.ID); err != nil {
				return err
			}
			if err := s.checkSubnetConflict(ctx, tx, e); err != nil {
				return err
			}
			fields := map[string]any{"status": models.ExamActive}
			if e.StartsAt.After(now) {
				fields["starts_at"] = now
			}
			return tx.UpdateExam(ctx, e.ID, fields)
		case "close":
			if e.Status != models.ExamActive && e.Status != models.ExamScheduled {
				return ErrInvalidState
			}
			fields := map[string]any{"status": models.ExamClosed}
			if e.EndsAt.After(now) {
				fields["ends_at"] = now
			}
			closed = true
			return tx.UpdateExam(ctx, e.ID, fields)
		}
		return invalid("unknown action %q", action)
	})
	if err != nil {
		return nil, err
	}
	s.invalidate(ctx)
	if closed && s.OnClosed != nil {
		s.OnClosed(ctx, examID)
	}
	e, err := s.repo.ExamByID(ctx, examID)
	if err == nil {
		s.hub.Publish(e.ID, "exam", map[string]any{"status": e.Status, "starts_at": e.StartsAt, "ends_at": e.EndsAt})
	}
	return e, err
}

func requireQuestions(ctx context.Context, tx *repositories.Repo, examID int64) error {
	qs, err := tx.Questions(ctx, examID)
	if err != nil {
		return err
	}
	if len(qs) == 0 {
		return ErrNoQuestions
	}
	return nil
}

// checkSubnetConflict: two live exams may share a subnet only if both have
// PINs, because the PIN is the only thing that tells the gate which exam a
// student is entering.
func (s *ExamService) checkSubnetConflict(ctx context.Context, tx *repositories.Repo, e *models.Exam) error {
	mine, err := config.ParseCIDR(e.LANCIDR)
	if err != nil {
		return invalid("exam has an invalid lan_cidr")
	}
	open, err := tx.OpenExams(ctx, time.Now().UTC())
	if err != nil {
		return err
	}
	for _, o := range open {
		if o.ID == e.ID {
			continue
		}
		theirs, err := config.ParseCIDR(o.LANCIDR)
		if err != nil {
			continue
		}
		if overlaps(mine, theirs) && (!e.PinRequired() || !o.PinRequired()) {
			return ErrSubnetConflict
		}
	}
	return nil
}

func (s *ExamService) uniquePIN(ctx context.Context, examID int64) (string, error) {
	for i := 0; i < 20; i++ {
		pin := auth.NewPIN()
		used, err := s.repo.PinInUse(ctx, pin, examID)
		if err != nil {
			return "", err
		}
		if !used {
			return pin, nil
		}
	}
	return "", ErrPinInUse
}

// SetPIN rotates (enabled=true) or clears the exam PIN.
func (s *ExamService) SetPIN(ctx context.Context, e *models.Exam, enabled bool) (*models.Exam, error) {
	if e.Status == models.ExamClosed {
		return nil, ErrInvalidState
	}
	var pin *string
	if enabled {
		p, err := s.uniquePIN(ctx, e.ID)
		if err != nil {
			return nil, err
		}
		pin = &p
	} else if e.Status == models.ExamActive {
		e2 := *e
		e2.PinCode = nil
		if err := s.checkSubnetConflict(ctx, s.repo, &e2); err != nil {
			return nil, err
		}
	}
	if err := s.repo.UpdateExam(ctx, e.ID, map[string]any{"pin_code": pin}); err != nil {
		return nil, err
	}
	s.invalidate(ctx)
	return s.repo.ExamByID(ctx, e.ID)
}

func (s *ExamService) Delete(ctx context.Context, e *models.Exam) error {
	if e.Status == models.ExamActive || e.Status == models.ExamScheduled {
		return ErrInvalidState
	}
	if err := s.repo.DeleteExam(ctx, e.ID); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}

// AccessCard is what the invigilator projects for students.
type AccessCard struct {
	IP          string `json:"ip"`
	URL         string `json:"url"`
	PIN         string `json:"pin,omitempty"`
	QRURL       string `json:"qr_url"`
	QRPNGBase64 string `json:"qr_png_base64"`
	LANCIDR     string `json:"lan_cidr"`
	Status      string `json:"status"`
}

// AccessCard builds the card. requestHost (the Host the staff member used)
// is the fallback when no LAN address is configured or detectable; in open
// mode the public URL (PUBLIC_URL, else scheme://requestHost) is shown.
func (s *ExamService) AccessCard(e *models.Exam, requestHost, requestScheme string) (*AccessCard, error) {
	if s.cfg.PublicURL != "" || s.cfg.OpenAccess() {
		base := s.cfg.PublicURL
		if base == "" {
			base = requestScheme + "://" + requestHost
		}
		return s.card(e, base, hostOnly(base))
	}
	ip, err := netutil.AdvertiseIP(s.cfg.LANAdvertiseIP)
	if err != nil {
		h, _, splitErr := net.SplitHostPort(requestHost)
		if splitErr != nil {
			h = requestHost
		}
		if net.ParseIP(strings.Trim(h, "[]")) == nil {
			return nil, invalid("cannot determine the LAN address; set LAN_ADVERTISE_IP")
		}
		ip = strings.Trim(h, "[]")
	}
	port := s.cfg.PublicPort
	if port == 0 {
		if _, p, err := net.SplitHostPort(s.cfg.HTTPAddr); err == nil {
			port, _ = strconv.Atoi(p)
		}
	}
	scheme := "http"
	if s.cfg.TLS() || s.cfg.PublicScheme == "https" {
		scheme = "https"
	}
	return s.card(e, netutil.HostURL(scheme, ip, port), ip)
}

func (s *ExamService) card(e *models.Exam, url, host string) (*AccessCard, error) {
	card := &AccessCard{IP: host, URL: url, QRURL: url, LANCIDR: e.LANCIDR, Status: string(e.Status)}
	if e.PinRequired() {
		card.PIN = *e.PinCode
		card.QRURL = url + "/exam?pin=" + card.PIN
	}
	png, err := qrcode.Encode(card.QRURL, qrcode.Medium, 320)
	if err != nil {
		return nil, err
	}
	card.QRPNGBase64 = base64.StdEncoding.EncodeToString(png)
	return card, nil
}

// hostOnly returns the host part of a URL like https://exam.example.com.
func hostOnly(u string) string {
	u = strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
	if i := strings.IndexByte(u, '/'); i >= 0 {
		u = u[:i]
	}
	return u
}

// --- Questions ----------------------------------------------------------------

type QuestionInput struct {
	Kind          models.QuestionKind `json:"kind"`
	Prompt        string              `json:"prompt"`
	Options       []string            `json:"options"`
	CorrectOption *int                `json:"correct_option"`
	// GAP: accepted answers for each blank ("_____") in the prompt, in order.
	Blanks   [][]string `json:"blanks"`
	Marks    float64    `json:"marks"`
	Position int        `json:"position"`
}

func (in *QuestionInput) toModel() (*models.Question, error) {
	if in.Kind == "" {
		in.Kind = models.QuestionMCQ
	}
	in.Prompt = strings.TrimSpace(in.Prompt)
	if in.Prompt == "" || len(in.Prompt) > 10000 {
		return nil, invalid("prompt is required (max 10000 chars)")
	}
	if in.Marks == 0 {
		in.Marks = 1
	}
	if in.Marks < 0 || in.Marks > 100 {
		return nil, invalid("marks must be between 0 and 100")
	}
	q := &models.Question{Kind: in.Kind, Prompt: in.Prompt, Marks: in.Marks, Position: in.Position}
	switch in.Kind {
	case models.QuestionMCQ:
		var opts []string
		for _, o := range in.Options {
			if o = strings.TrimSpace(o); o != "" {
				opts = append(opts, o)
			}
		}
		if len(opts) < 2 || len(opts) > 8 {
			return nil, invalid("MCQ needs 2–8 non-empty options")
		}
		if in.CorrectOption == nil || *in.CorrectOption < 0 || *in.CorrectOption >= len(opts) {
			return nil, invalid("correct_option must index one of the options")
		}
		q.Options, q.CorrectOption = opts, in.CorrectOption
	case models.QuestionGap:
		n := models.CountBlanks(in.Prompt)
		if n == 0 || n > 10 {
			return nil, invalid("a fill-in-the-gap question needs 1–10 blanks written as _____ (three or more underscores)")
		}
		if len(in.Blanks) != n {
			return nil, invalid("the question has %d blank(s) but %d answer(s) were given", n, len(in.Blanks))
		}
		blanks := make([][]string, n)
		for i, alts := range in.Blanks {
			for _, a := range alts {
				if a = strings.TrimSpace(a); a != "" && len(a) <= 200 {
					blanks[i] = append(blanks[i], a)
				}
			}
			if len(blanks[i]) == 0 || len(blanks[i]) > 10 {
				return nil, invalid("blank %d needs 1–10 accepted answers", i+1)
			}
		}
		q.Options, q.Blanks = nil, blanks
	case models.QuestionText:
		q.Options = nil
	default:
		return nil, invalid("kind must be MCQ, TEXT or GAP")
	}
	return q, nil
}

func (s *ExamService) AddQuestions(ctx context.Context, e *models.Exam, ins []QuestionInput) ([]models.Question, error) {
	if e.Status == models.ExamActive || e.Status == models.ExamClosed {
		return nil, invalid("questions are locked once the exam is open")
	}
	if len(ins) == 0 || len(ins) > 500 {
		return nil, invalid("provide 1–500 questions")
	}
	next, err := s.repo.NextQuestionPosition(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	qs := make([]models.Question, 0, len(ins))
	for i := range ins {
		q, err := ins[i].toModel()
		if err != nil {
			return nil, invalid("question %d: %s", i+1, err.Error())
		}
		q.ExamID = e.ID
		if q.Position == 0 {
			q.Position = next
			next++
		}
		qs = append(qs, *q)
	}
	if err := s.repo.CreateQuestions(ctx, qs); err != nil {
		return nil, err
	}
	return qs, nil
}

func (s *ExamService) UpdateQuestion(ctx context.Context, e *models.Exam, qid int64, in QuestionInput) error {
	if e.Status == models.ExamActive || e.Status == models.ExamClosed {
		return invalid("questions are locked once the exam is open")
	}
	q, err := in.toModel()
	if err != nil {
		return err
	}
	return s.repo.UpdateQuestion(ctx, e.ID, qid, q)
}

func (s *ExamService) DeleteQuestion(ctx context.Context, e *models.Exam, qid int64) error {
	if e.Status == models.ExamActive || e.Status == models.ExamClosed {
		return invalid("questions are locked once the exam is open")
	}
	return s.repo.DeleteQuestion(ctx, e.ID, qid)
}

// SetProctoring changes an exam's proctoring switches. It is allowed while
// the exam is running: students pick changes up on their next heartbeat
// (≤ 30 s), and the server stops accepting disabled events at once.
func (s *ExamService) SetProctoring(ctx context.Context, e *models.Exam, p models.Proctoring) (*models.Exam, error) {
	if e.Status == models.ExamClosed {
		return nil, ErrInvalidState
	}
	if err := s.repo.SetExamProctoring(ctx, e.ID, p.Settings()); err != nil {
		return nil, err
	}
	s.invalidate(ctx)
	out, err := s.repo.ExamByID(ctx, e.ID)
	if err == nil {
		s.hub.Publish(e.ID, "exam", map[string]any{"status": out.Status, "proctoring": out.Proctoring.Resolve()})
	}
	return out, err
}
