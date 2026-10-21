package questionbank

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"examshield/internal/examimport"
)

// The bank is only useful if the importer reads back exactly what was
// written: every question, option, answer letter and mark.
func TestBankRoundTripsThroughImporter(t *testing.T) {
	cases := map[string][]Subject{"all": Subjects}
	for _, s := range Subjects {
		cases[s.Slug] = []Subject{s}
	}
	for name, subjects := range cases {
		for _, answers := range []bool{true, false} {
			pdf := Render(subjects, Options{Title: "Question Bank", Subtitle: name, Answers: answers})
			lines, pages, err := examimport.ExtractLines(pdf)
			require.NoError(t, err, name)
			res := examimport.Parse(lines)
			assert.Empty(t, res.Warnings, name)

			var want []Item
			for _, its := range Items(subjects) {
				want = append(want, its...)
			}
			require.Len(t, res.Questions, len(want), "%s answers=%v pages=%d", name, answers, pages)
			for i, w := range want {
				got := res.Questions[i]
				assert.Equal(t, w.Number, got.Number, name)
				assert.Equal(t, w.Kind, got.Kind, "%s Q%d", name, w.Number)
				assert.Equal(t, w.Prompt, got.Prompt, "%s Q%d", name, w.Number)
				assert.Equal(t, w.Marks, got.Marks, "%s Q%d", name, w.Number)
				if w.Kind == "MCQ" {
					assert.Equal(t, w.Options, got.Options, "%s Q%d", name, w.Number)
					if answers {
						require.NotNil(t, got.CorrectOption, "%s Q%d", name, w.Number)
						assert.Equal(t, w.Answer, *got.CorrectOption, "%s Q%d", name, w.Number)
						assert.Empty(t, got.Warnings, "%s Q%d", name, w.Number)
					} else {
						assert.Nil(t, got.CorrectOption, "student paper must not reveal answers")
					}
				}
			}
		}
	}
}

func TestBankShape(t *testing.T) {
	require.Len(t, Subjects, 11)
	letters := map[int]int{}
	for _, s := range Subjects {
		assert.Len(t, s.MCQs, 8, s.Name)
		assert.Len(t, s.Written, 2, s.Name)
		for _, q := range s.MCQs {
			assert.Len(t, q.Options, 4, q.Q)
			for _, o := range q.Options {
				assert.Equal(t, strings.TrimSpace(o), o)
			}
		}
	}
	for _, its := range Items(Subjects) {
		for _, it := range its {
			if it.Kind == "MCQ" {
				letters[it.Answer]++
			}
		}
	}
	// Answers must not all sit on one letter.
	for l := 0; l < 4; l++ {
		assert.Greater(t, letters[l], 10, "answer letter %c used %d times", 'A'+l, letters[l])
	}
}

// Presentation quality: a page must never start in the middle of a question
// (options or answer separated from their prompt).
func TestNoQuestionSplitAcrossPages(t *testing.T) {
	for _, answers := range []bool{true, false} {
		pdf := Render(Subjects, Options{Title: "Question Bank", Answers: answers})
		lines, _, err := examimport.ExtractLines(pdf)
		require.NoError(t, err)
		for i, l := range lines {
			if !strings.Contains(l, " Page ") || i+1 >= len(lines) {
				continue
			}
			next := lines[i+1]
			ok := strings.HasPrefix(next, "Section ") || (len(next) > 2 && next[0] >= '1' && next[0] <= '9' && strings.Contains(next[:5], "."))
			assert.True(t, ok, "page starts mid-question with %q (answers=%v)", next, answers)
		}
	}
}
