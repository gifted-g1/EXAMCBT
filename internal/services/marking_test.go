package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"examshield/internal/models"
)

func TestKeywordSuggest(t *testing.T) {
	q := models.Question{Kind: models.QuestionText, Marks: 10, MarkingGuide: []models.MarkingPoint{
		{Point: "Defines encryption", Marks: 3, Keywords: []string{"encrypt", "ciphertext"}},
		{Point: "Symmetric vs asymmetric", Marks: 4, Keywords: []string{"public key", "symmetric"}},
		{Point: "Example", Marks: 3, Keywords: []string{"AES", "RSA"}},
	}}

	s := KeywordSuggest(q, "Encryption encrypts data into CIPHERTEXT. Public-key systems such as RSA use two keys.")
	assert.Equal(t, 10.0, s.Marks)
	assert.True(t, s.Points[0].Met)
	assert.ElementsMatch(t, []string{"encrypt", "ciphertext"}, s.Points[0].Matched, "stems and case are ignored")
	assert.True(t, s.Points[1].Met, "multi-word keyword matches across punctuation")
	assert.Contains(t, s.Highlights, "RSA")

	s = KeywordSuggest(q, "Scrambling data so only the holder of the key can read it.")
	assert.Equal(t, 0.0, s.Marks, "no keywords: suggestion is 0 and the examiner decides")

	s = KeywordSuggest(q, "unsymmetrical")
	assert.False(t, s.Points[1].Met, "keywords match whole words only")

	// Points worth more than the question are capped.
	q.Marks = 5
	assert.Equal(t, 5.0, KeywordSuggest(q, "encrypt symmetric AES").Marks)

	assert.Nil(t, KeywordSuggest(models.Question{Kind: models.QuestionText, Marks: 5}, "anything"), "no guide, no suggestion")
}

func TestMarkAnswerUsesExaminerMark(t *testing.T) {
	q := models.Question{Kind: models.QuestionText, Marks: 5}
	st, aw := markAnswer(q, &models.Answer{Response: "text"})
	assert.Equal(t, StatusNeedsMarking, st)
	assert.Nil(t, aw)

	m := 3.5
	st, aw = markAnswer(q, &models.Answer{Response: "text", AwardedMarks: &m})
	assert.Equal(t, StatusMarked, st)
	assert.Equal(t, 3.5, *aw)

	st, _ = markAnswer(q, nil)
	assert.Equal(t, StatusUnanswered, st)
}

func TestGapMarking(t *testing.T) {
	q := models.Question{Kind: models.QuestionGap, Marks: 3, Blanks: [][]string{{"SYN"}, {"SYN-ACK", "SYN ACK"}, {"ACK"}}}
	st, aw := markAnswer(q, &models.Answer{Response: `[" syn ","syn  ack","ACK."]`})
	assert.Equal(t, StatusCorrect, st, "case, spacing and trailing full stop are ignored")
	assert.Equal(t, 3.0, *aw)

	st, aw = markAnswer(q, &models.Answer{Response: `["SYN","FIN",""]`})
	assert.Equal(t, StatusPartial, st)
	assert.Equal(t, 1.0, *aw)

	st, aw = markAnswer(q, &models.Answer{Response: `["x"]`})
	assert.Equal(t, StatusIncorrect, st)
	assert.Equal(t, 0.0, *aw)

	st, _ = markAnswer(q, &models.Answer{Response: `["",""]`})
	assert.Equal(t, StatusUnanswered, st)
}

func TestExamDeadline(t *testing.T) {
	start := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	e := models.Exam{EndsAt: start.Add(time.Hour), DurationMinutes: 25}
	assert.Equal(t, start.Add(25*time.Minute), e.Deadline(start))
	assert.Equal(t, e.EndsAt, e.Deadline(start.Add(50*time.Minute)), "never past the window")
	e.DurationMinutes = 0
	assert.Equal(t, e.EndsAt, e.Deadline(start))
}
