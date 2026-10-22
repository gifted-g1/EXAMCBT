package services

import (
	"context"
	"strconv"
	"strings"

	"examshield/internal/models"
)

// Answer status values shown to examiners.
const (
	StatusCorrect      = "correct"
	StatusIncorrect    = "incorrect"
	StatusUnanswered   = "unanswered"
	StatusNeedsMarking = "needs_marking"
)

// markAnswer is the single marking rule used by grading, answer sheets and
// question analysis, so they can never disagree. Written answers are never
// auto-marked: awarded stays nil until an examiner confirms a mark.
func markAnswer(q models.Question, a *models.Answer) (status string, awarded *float64) {
	zero := 0.0
	if a == nil || strings.TrimSpace(a.Response) == "" {
		return StatusUnanswered, &zero
	}
	if q.Kind != models.QuestionMCQ || q.CorrectOption == nil {
		if a.AwardedMarks != nil {
			m := *a.AwardedMarks
			return StatusMarked, &m
		}
		return StatusNeedsMarking, nil
	}
	if a.Response == strconv.Itoa(*q.CorrectOption) {
		m := q.Marks
		return StatusCorrect, &m
	}
	return StatusIncorrect, &zero
}

// ScriptItem is one question on a student's answer sheet.
type ScriptItem struct {
	QuestionID    int64               `json:"question_id"`
	Number        int                 `json:"number"`
	Kind          models.QuestionKind `json:"kind"`
	Prompt        string              `json:"prompt"`
	Options       []string            `json:"options,omitempty"`
	CorrectOption *int                `json:"correct_option,omitempty"`
	Response      string              `json:"response"`
	Status        string              `json:"status"`
	Marks         float64             `json:"marks"`
	Awarded       *float64            `json:"awarded"`
	Comment       string              `json:"comment,omitempty"` // examiner's note on a written answer
}

// AnswerScript is a student's marked answer sheet. Staff only: it contains
// the correct answers.
type AnswerScript struct {
	SessionID int64                `json:"session_id"`
	ExamID    int64                `json:"exam_id"`
	ExamTitle string               `json:"exam_title"`
	MatricNo  string               `json:"matric_no"`
	FullName  string               `json:"full_name"`
	Status    models.SessionStatus `json:"status"`
	Score     float64              `json:"score"`
	MaxScore  float64              `json:"max_score"`
	// Marks available on questions marked so far (MCQ + marked written).
	AutoMarkedMax float64        `json:"auto_marked_max"`
	Counts        map[string]int `json:"counts"`
	Items         []ScriptItem   `json:"items"`
}

func (s *ProctorService) AnswerScript(ctx context.Context, sessionID int64) (*AnswerScript, error) {
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
	byQ := answersByQuestion(ans)
	out := &AnswerScript{SessionID: sess.ID, ExamID: e.ID, ExamTitle: e.Title, Status: sess.Status,
		Counts: map[string]int{StatusCorrect: 0, StatusIncorrect: 0, StatusUnanswered: 0, StatusNeedsMarking: 0, StatusMarked: 0},
		Items:  []ScriptItem{}}
	if sess.Student != nil {
		out.MatricNo, out.FullName = sess.Student.MatricNo, sess.Student.FullName
	}
	for i, q := range qs {
		a := byQ[q.ID]
		st, awarded := markAnswer(q, a)
		it := ScriptItem{QuestionID: q.ID, Number: i + 1, Kind: q.Kind, Prompt: q.Prompt,
			Options: q.Options, CorrectOption: q.CorrectOption, Status: st, Marks: q.Marks, Awarded: awarded}
		if a != nil {
			it.Response, it.Comment = a.Response, a.MarkerComment
		}
		out.Items = append(out.Items, it)
		out.Counts[st]++
		out.MaxScore += q.Marks
		if q.Kind == models.QuestionMCQ || st == StatusMarked {
			out.AutoMarkedMax += q.Marks
		}
		if awarded != nil {
			out.Score += *awarded
		}
	}
	return out, nil
}

func answersByQuestion(ans []models.Answer) map[int64]*models.Answer {
	m := make(map[int64]*models.Answer, len(ans))
	for i := range ans {
		m[ans[i].QuestionID] = &ans[i]
	}
	return m
}

// QuestionStat is the class-wide result for one question.
type QuestionStat struct {
	QuestionID    int64               `json:"question_id"`
	Number        int                 `json:"number"`
	Kind          models.QuestionKind `json:"kind"`
	Prompt        string              `json:"prompt"`
	Options       []string            `json:"options,omitempty"`
	CorrectOption *int                `json:"correct_option,omitempty"`
	Correct       int                 `json:"correct"`
	Incorrect     int                 `json:"incorrect"`
	Unanswered    int                 `json:"unanswered"`
	Answered      int                 `json:"answered"`
	OptionCounts  []int               `json:"option_counts,omitempty"` // students per option (MCQ)
	CommonWrong   *int                `json:"common_wrong,omitempty"`  // most chosen wrong option
	PercentRight  *float64            `json:"percent_right"`
}

// ItemAnalysis summarises how every student who started the exam did on
// each question.
func (s *ProctorService) ItemAnalysis(ctx context.Context, e *models.Exam) ([]QuestionStat, int, error) {
	qs, err := s.repo.Questions(ctx, e.ID)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.repo.SessionsForExam(ctx, e.ID)
	if err != nil {
		return nil, 0, err
	}
	ans, err := s.repo.AnswersForExam(ctx, e.ID)
	if err != nil {
		return nil, 0, err
	}
	type key struct{ s, q int64 }
	resp := make(map[key]*models.Answer, len(ans))
	for i := range ans {
		resp[key{ans[i].SessionID, ans[i].QuestionID}] = &ans[i]
	}
	stats := make([]QuestionStat, len(qs))
	for i, q := range qs {
		st := QuestionStat{QuestionID: q.ID, Number: i + 1, Kind: q.Kind, Prompt: q.Prompt, Options: q.Options, CorrectOption: q.CorrectOption}
		if q.Kind == models.QuestionMCQ {
			st.OptionCounts = make([]int, len(q.Options))
		}
		for _, r := range rows {
			a := resp[key{r.ID, q.ID}]
			status, _ := markAnswer(q, a)
			switch status {
			case StatusCorrect:
				st.Correct++
			case StatusIncorrect:
				st.Incorrect++
			case StatusUnanswered:
				st.Unanswered++
			}
			if status != StatusUnanswered {
				st.Answered++
				if idx, err := strconv.Atoi(a.Response); err == nil && idx >= 0 && idx < len(st.OptionCounts) {
					st.OptionCounts[idx]++
				}
			}
		}
		if q.Kind == models.QuestionMCQ && len(rows) > 0 {
			p := round2(float64(st.Correct) / float64(len(rows)) * 100)
			st.PercentRight = &p
			best := -1
			for oi, n := range st.OptionCounts {
				if q.CorrectOption != nil && oi == *q.CorrectOption {
					continue
				}
				if n > 0 && (best < 0 || n > st.OptionCounts[best]) {
					best = oi
				}
			}
			if best >= 0 {
				b := best
				st.CommonWrong = &b
			}
		}
		stats[i] = st
	}
	return stats, len(rows), nil
}
