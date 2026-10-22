package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"examshield/internal/models"
	"examshield/internal/repositories"
)

// Theory marking. Written answers are never scored by the machine: the
// examiner writes a marking guide (key points, marks, keywords), the app
// suggests a mark from keyword matches and — only when switched on — from
// an AI model, and a human confirms every mark.

var ErrAIMarkingOff = errors.New("AI-suggested marks are turned off on this server")

const (
	maxGuidePoints   = 20
	maxKeywords      = 15
	maxAIBatch       = 40
	aiConcurrency    = 4
	SuggestKeyword   = "keyword"
	SuggestAI        = "ai"
	StatusMarked     = "marked"
	CandidateLabelFm = "Candidate %d"
)

// --- Keyword suggestion ---------------------------------------------------------

// PointMatch reports whether one marking-guide point was found in an answer.
type PointMatch struct {
	Index   int      `json:"index"`
	Point   string   `json:"point"`
	Marks   float64  `json:"marks"`
	Met     bool     `json:"met"`
	Matched []string `json:"matched"`
}

// KeywordSuggestion is the keyword-based suggested mark for one answer.
type KeywordSuggestion struct {
	Marks  float64      `json:"marks"`
	Max    float64      `json:"max"`
	Points []PointMatch `json:"points"`
	// Keywords found in the answer, for highlighting in the marking screen.
	Highlights []string `json:"highlights"`
}

var wordRe = regexp.MustCompile(`[\p{L}\p{N}]+`)

// stem strips a few common English endings so "encrypts", "encrypted" and
// "encrypting" all match the keyword "encrypt". Deliberately light: the
// examiner checks every suggestion.
func stem(w string) string {
	for _, suf := range []string{"ing", "ies", "ied", "es", "ed", "ly", "s"} {
		if len(w) > len(suf)+2 && strings.HasSuffix(w, suf) {
			base := w[:len(w)-len(suf)]
			if suf == "ies" || suf == "ied" {
				base += "y"
			}
			return base
		}
	}
	return w
}

func stems(s string) []string {
	ws := wordRe.FindAllString(strings.ToLower(s), -1)
	for i, w := range ws {
		ws[i] = stem(w)
	}
	return ws
}

// containsSeq reports whether needle occurs as consecutive words in hay.
func containsSeq(hay, needle []string) bool {
	if len(needle) == 0 || len(needle) > len(hay) {
		return false
	}
outer:
	for i := 0; i+len(needle) <= len(hay); i++ {
		for j, n := range needle {
			if hay[i+j] != n {
				continue outer
			}
		}
		return true
	}
	return false
}

// KeywordSuggest matches an answer against a question's marking guide. A
// point is met when any of its keywords (or, with none, the point text
// itself) appears as whole words. Returns nil when there is no guide.
func KeywordSuggest(q models.Question, response string) *KeywordSuggestion {
	if len(q.MarkingGuide) == 0 {
		return nil
	}
	words := stems(response)
	out := &KeywordSuggestion{Max: q.Marks, Points: make([]PointMatch, 0, len(q.MarkingGuide)), Highlights: []string{}}
	seen := map[string]bool{}
	for i, p := range q.MarkingGuide {
		pm := PointMatch{Index: i, Point: p.Point, Marks: p.Marks, Matched: []string{}}
		kws := p.Keywords
		if len(kws) == 0 {
			kws = []string{p.Point}
		}
		for _, kw := range kws {
			if containsSeq(words, stems(kw)) {
				pm.Met = true
				pm.Matched = append(pm.Matched, kw)
				if k := strings.ToLower(kw); !seen[k] {
					seen[k] = true
					out.Highlights = append(out.Highlights, kw)
				}
			}
		}
		if pm.Met {
			out.Marks += p.Marks
		}
		out.Points = append(out.Points, pm)
	}
	out.Marks = round2(math.Min(out.Marks, q.Marks))
	return out
}

// --- AI suggestion --------------------------------------------------------------

// AIMarkRequest is what the AI marker sees: the question and the answer
// text only — never the student's name, matric number or photo.
type AIMarkRequest struct {
	Question    string
	MaxMarks    float64
	Guide       []models.MarkingPoint
	ModelAnswer string
	Answer      string
}

// AISuggestion is a suggested mark with the model's reasoning.
type AISuggestion struct {
	Marks     float64   `json:"marks"`
	Reason    string    `json:"reason"`
	PointsMet []int     `json:"points_met"`
	Model     string    `json:"model"`
	At        time.Time `json:"at"`
}

// AIMarker suggests marks for written answers. The production
// implementation calls Claude; tests use a fake.
type AIMarker interface {
	Suggest(ctx context.Context, req AIMarkRequest) (*AISuggestion, error)
}

// --- Service ----------------------------------------------------------------------

type MarkingService struct {
	repo *repositories.Repo
	ai   AIMarker // nil when AI marking is off
}

func NewMarkingService(repo *repositories.Repo, ai AIMarker) *MarkingService {
	return &MarkingService{repo: repo, ai: ai}
}

func (s *MarkingService) AIEnabled() bool { return s.ai != nil }

type GuideInput struct {
	Points      []models.MarkingPoint `json:"points"`
	ModelAnswer string                `json:"model_answer"`
}

// SetGuide saves the marking guide for a written question. Allowed at any
// time, including after the exam has run, since it only affects marking.
func (s *MarkingService) SetGuide(ctx context.Context, e *models.Exam, qid int64, in GuideInput) (*models.Question, error) {
	q, err := questionIn(ctx, s.repo, e.ID, qid)
	if err != nil {
		return nil, err
	}
	if q.Kind != models.QuestionText {
		return nil, invalid("marking guides are for written (TEXT) questions; multiple choice is marked automatically")
	}
	if len(in.Points) > maxGuidePoints {
		return nil, invalid("a marking guide can have at most %d points", maxGuidePoints)
	}
	var total float64
	pts := make([]models.MarkingPoint, 0, len(in.Points))
	for i, p := range in.Points {
		p.Point = strings.TrimSpace(p.Point)
		if p.Point == "" || len(p.Point) > 500 {
			return nil, invalid("point %d: describe the point (max 500 characters)", i+1)
		}
		if p.Marks <= 0 || p.Marks > q.Marks {
			return nil, invalid("point %d: marks must be more than 0 and at most %g", i+1, q.Marks)
		}
		var kws []string
		for _, k := range p.Keywords {
			if k = strings.TrimSpace(k); k != "" && len(k) <= 100 {
				kws = append(kws, k)
			}
		}
		if len(kws) > maxKeywords {
			return nil, invalid("point %d: at most %d keywords", i+1, maxKeywords)
		}
		p.Keywords = kws
		total += p.Marks
		pts = append(pts, p)
	}
	if total > q.Marks+0.001 {
		return nil, invalid("the points add up to %g marks but the question is worth %g", round2(total), q.Marks)
	}
	ma := strings.TrimSpace(in.ModelAnswer)
	if len(ma) > 10000 {
		return nil, invalid("model answer is too long (max 10000 characters)")
	}
	q.MarkingGuide, q.ModelAnswer = pts, ma
	if err := s.repo.SetMarkingGuide(ctx, q); err != nil {
		return nil, err
	}
	return q, nil
}

func questionIn(ctx context.Context, repo *repositories.Repo, examID, qid int64) (*models.Question, error) {
	qs, err := repo.Questions(ctx, examID)
	if err != nil {
		return nil, err
	}
	for i := range qs {
		if qs[i].ID == qid {
			return &qs[i], nil
		}
	}
	return nil, repositories.ErrNotFound
}

// MarkingQuestion summarises marking progress for one written question.
type MarkingQuestion struct {
	QuestionID  int64                 `json:"question_id"`
	Number      int                   `json:"number"`
	Prompt      string                `json:"prompt"`
	Marks       float64               `json:"marks"`
	Guide       []models.MarkingPoint `json:"marking_guide"`
	ModelAnswer string                `json:"model_answer"`
	Answered    int                   `json:"answered"`
	Marked      int                   `json:"marked"`
}

type MarkingOverview struct {
	AIEnabled bool              `json:"ai_enabled"`
	Questions []MarkingQuestion `json:"questions"`
}

// Overview lists the exam's written questions with marking progress.
func (s *MarkingService) Overview(ctx context.Context, e *models.Exam) (*MarkingOverview, error) {
	qs, err := s.repo.Questions(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	ans, err := s.repo.AnswersForExam(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	out := &MarkingOverview{AIEnabled: s.AIEnabled(), Questions: []MarkingQuestion{}}
	idx := map[int64]int{}
	for i, q := range qs {
		if q.Kind != models.QuestionText {
			continue
		}
		g := []models.MarkingPoint(q.MarkingGuide)
		if g == nil {
			g = []models.MarkingPoint{}
		}
		idx[q.ID] = len(out.Questions)
		out.Questions = append(out.Questions, MarkingQuestion{QuestionID: q.ID, Number: i + 1, Prompt: q.Prompt,
			Marks: q.Marks, Guide: g, ModelAnswer: q.ModelAnswer})
	}
	for _, a := range ans {
		i, ok := idx[a.QuestionID]
		if !ok || strings.TrimSpace(a.Response) == "" {
			continue
		}
		out.Questions[i].Answered++
		if a.AwardedMarks != nil {
			out.Questions[i].Marked++
		}
	}
	return out, nil
}

// MarkingScript is one candidate's answer to the question being marked.
// Candidates are anonymous ("Candidate 3") so marking is not biased by
// who wrote the answer.
type MarkingScript struct {
	AnswerID   int64              `json:"answer_id"`
	Candidate  string             `json:"candidate"`
	Response   string             `json:"response"`
	Awarded    *float64           `json:"awarded"`
	Comment    string             `json:"comment"`
	MarkedAt   *time.Time         `json:"marked_at"`
	Keyword    *KeywordSuggestion `json:"keyword_suggestion"`
	AI         *AISuggestion      `json:"ai_suggestion"`
	SessionEnd bool               `json:"submitted"`
}

type QuestionScripts struct {
	Question MarkingQuestion `json:"question"`
	Scripts  []MarkingScript `json:"scripts"`
}

// Scripts returns every written answer to one question with suggestions.
func (s *MarkingService) Scripts(ctx context.Context, e *models.Exam, qid int64) (*QuestionScripts, error) {
	ov, err := s.Overview(ctx, e)
	if err != nil {
		return nil, err
	}
	var mq *MarkingQuestion
	for i := range ov.Questions {
		if ov.Questions[i].QuestionID == qid {
			mq = &ov.Questions[i]
		}
	}
	if mq == nil {
		return nil, repositories.ErrNotFound
	}
	q, err := questionIn(ctx, s.repo, e.ID, qid)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.SessionsForExam(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	ans, err := s.repo.AnswersForExam(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	// Candidate numbers follow session order, stable across questions.
	label := map[int64]int{}
	ended := map[int64]bool{}
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
		ended[r.ID] = r.Status == models.SessionSubmitted || r.Status == models.SessionClosed
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for i, id := range ids {
		label[id] = i + 1
	}
	out := &QuestionScripts{Question: *mq, Scripts: []MarkingScript{}}
	num := map[int64]int{} // answer ID -> candidate number
	for _, a := range ans {
		if a.QuestionID != qid || strings.TrimSpace(a.Response) == "" {
			continue
		}
		num[a.ID] = label[a.SessionID]
		out.Scripts = append(out.Scripts, MarkingScript{AnswerID: a.ID, Candidate: fmt.Sprintf(CandidateLabelFm, label[a.SessionID]),
			Response: a.Response, Awarded: a.AwardedMarks, Comment: a.MarkerComment, MarkedAt: a.MarkedAt,
			Keyword: KeywordSuggest(*q, a.Response), AI: suggestionFrom(a.Suggestion), SessionEnd: ended[a.SessionID]})
	}
	sort.Slice(out.Scripts, func(i, j int) bool {
		return num[out.Scripts[i].AnswerID] < num[out.Scripts[j].AnswerID]
	})
	return out, nil
}

func suggestionFrom(m map[string]any) *AISuggestion {
	if len(m) == 0 {
		return nil
	}
	// Round-trip through JSON: stored numbers come back as json.Number.
	b, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	var out AISuggestion
	if json.Unmarshal(b, &out) != nil {
		return nil
	}
	return &out
}

type MarkResult struct {
	AnswerID     int64    `json:"answer_id"`
	Awarded      *float64 `json:"awarded"`
	SessionID    int64    `json:"session_id"`
	SessionScore *float64 `json:"session_score"`
}

// Mark records the examiner's mark (nil clears it) and, for a finished
// session, recomputes the student's total score.
func (s *MarkingService) Mark(ctx context.Context, e *models.Exam, answerID int64, marks *float64, comment string, by int64) (*MarkResult, error) {
	if len(comment) > 2000 {
		return nil, invalid("comment is too long (max 2000 characters)")
	}
	res := &MarkResult{AnswerID: answerID, Awarded: marks}
	err := s.repo.Tx(ctx, func(tx *repositories.Repo) error {
		a, err := tx.AnswerByID(ctx, answerID)
		if err != nil {
			return err
		}
		sess, err := tx.SessionByIDForUpdate(ctx, a.SessionID)
		if err != nil {
			return err
		}
		if sess.ExamID != e.ID {
			return repositories.ErrNotFound
		}
		q, err := questionIn(ctx, tx, e.ID, a.QuestionID)
		if err != nil {
			return err
		}
		if q.Kind != models.QuestionText {
			return invalid("multiple-choice answers are marked automatically")
		}
		if marks != nil {
			m := round2(*marks)
			if m < 0 || m > q.Marks {
				return invalid("marks must be between 0 and %g", q.Marks)
			}
			marks = &m
			res.Awarded = marks
		}
		if err := tx.SetAnswerMark(ctx, a.ID, marks, strings.TrimSpace(comment), by); err != nil {
			return err
		}
		res.SessionID = sess.ID
		if sess.Score != nil { // finished session: keep its total in step
			score, err := grade(ctx, tx, sess)
			if err != nil {
				return err
			}
			if err := tx.SetScore(ctx, sess.ID, score); err != nil {
				return err
			}
			res.SessionScore = &score
		}
		return nil
	})
	return res, err
}

type AISuggestResult struct {
	Requested int      `json:"requested"`
	Suggested int      `json:"suggested"`
	Failed    int      `json:"failed"`
	Errors    []string `json:"errors,omitempty"`
}

// SuggestAI asks the AI marker for a suggested mark on each unmarked
// answer to a question (or only answerIDs, when given). Suggestions are
// stored for the examiner to accept or change; nothing is awarded.
func (s *MarkingService) SuggestAI(ctx context.Context, e *models.Exam, qid int64, answerIDs []int64) (*AISuggestResult, error) {
	if s.ai == nil {
		return nil, ErrAIMarkingOff
	}
	q, err := questionIn(ctx, s.repo, e.ID, qid)
	if err != nil {
		return nil, err
	}
	if q.Kind != models.QuestionText {
		return nil, invalid("AI suggestions are only for written questions")
	}
	if len(q.MarkingGuide) == 0 && q.ModelAnswer == "" {
		return nil, invalid("add a marking guide or model answer first; the AI marks against it")
	}
	ans, err := s.repo.AnswersForExam(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	want := map[int64]bool{}
	for _, id := range answerIDs {
		want[id] = true
	}
	var todo []models.Answer
	for _, a := range ans {
		if a.QuestionID != qid || strings.TrimSpace(a.Response) == "" {
			continue
		}
		if len(want) > 0 && !want[a.ID] {
			continue
		}
		if len(want) == 0 && a.AwardedMarks != nil {
			continue // already marked by a human
		}
		todo = append(todo, a)
	}
	if len(todo) > maxAIBatch {
		todo = todo[:maxAIBatch]
	}
	res := &AISuggestResult{Requested: len(todo)}
	var mu sync.Mutex
	sem := make(chan struct{}, aiConcurrency)
	var wg sync.WaitGroup
	for _, a := range todo {
		wg.Add(1)
		sem <- struct{}{}
		go func(a models.Answer) {
			defer wg.Done()
			defer func() { <-sem }()
			sug, err := s.ai.Suggest(ctx, AIMarkRequest{Question: q.Prompt, MaxMarks: q.Marks, Guide: q.MarkingGuide,
				ModelAnswer: q.ModelAnswer, Answer: a.Response})
			if err == nil {
				sug.Marks = round2(math.Max(0, math.Min(sug.Marks, q.Marks)))
				err = s.repo.SetAnswerSuggestion(ctx, a.ID, map[string]any{"marks": sug.Marks, "reason": sug.Reason,
					"points_met": sug.PointsMet, "model": sug.Model, "at": sug.At.UTC().Format(time.RFC3339)})
			}
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				res.Failed++
				if len(res.Errors) < 3 {
					res.Errors = append(res.Errors, err.Error())
				}
				return
			}
			res.Suggested++
		}(a)
	}
	wg.Wait()
	return res, nil
}
