package examimport

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func idx(i int) *int { return &i }

func TestSamplePDFRoundTrip(t *testing.T) {
	pdf := SamplePDF()
	lines, pages, err := ExtractLines(pdf)
	require.NoError(t, err)
	assert.Equal(t, 1, pages)
	t.Logf("extracted:\n%s", strings.Join(lines, "\n"))

	res := Parse(lines)
	require.Len(t, res.Questions, 6, "warnings: %v", res.Warnings)
	assert.Empty(t, res.Warnings)

	q := res.Questions
	assert.Equal(t, "MCQ", q[0].Kind)
	assert.Equal(t, "Which layer of the OSI model is responsible for routing packets?", q[0].Prompt)
	assert.Equal(t, []string{"Data link", "Network", "Transport", "Session"}, q[0].Options)
	assert.Equal(t, idx(1), q[0].CorrectOption)

	assert.Equal(t, 2.0, q[1].Marks, "[2 marks] parsed and stripped")
	assert.Equal(t, "What does CIDR stand for?", q[1].Prompt)

	assert.Equal(t, []string{"256", "255", "254", "128"}, q[2].Options, "inline options split")
	assert.Equal(t, idx(2), q[2].CorrectOption)

	assert.Equal(t, []string{"403", "409", "423", "451"}, q[3].Options, "(a) style options")
	assert.Nil(t, q[3].CorrectOption)
	assert.Contains(t, q[3].Warnings[0], "no answer")

	assert.Equal(t, idx(1), q[4].CorrectOption, "answer key section")

	assert.Equal(t, "TEXT", q[5].Kind)
	assert.Equal(t, 5.0, q[5].Marks)
	assert.Equal(t, "In two sentences, explain why an exam server should only listen on the LAN interface.", q[5].Prompt)
}

func TestParseVariants(t *testing.T) {
	res := Parse([]string{
		"Department of Computing", "Page 1 of 3",
		"Q1. Pick the prime", "a) 4", "b) 7", "Ans - b",
		"Question 2: Describe TCP", "Marks: 3",
		"3) Capital of Nigeria?", "A. Lagos", "B. Abuja", "continued text for B", "Correct answer: B",
		"4. One option only", "A. Yes",
		"5. Bad answer", "A. x", "B. y", "Answer: F",
	})
	require.Len(t, res.Questions, 5)
	assert.Equal(t, idx(1), res.Questions[0].CorrectOption)
	assert.Equal(t, "TEXT", res.Questions[1].Kind)
	assert.Equal(t, 3.0, res.Questions[1].Marks)
	assert.Equal(t, "Abuja continued text for B", res.Questions[2].Options[1])
	assert.Equal(t, idx(1), res.Questions[2].CorrectOption)
	assert.NotEmpty(t, res.Questions[3].Warnings)
	assert.Nil(t, res.Questions[4].CorrectOption)
	assert.Contains(t, res.Questions[4].Warnings[0], "not one of the options")
}

func TestNoQuestionsAndBadFiles(t *testing.T) {
	res := Parse([]string{"Just a letter", "with no numbering"})
	assert.Empty(t, res.Questions)
	assert.NotEmpty(t, res.Warnings)

	_, _, err := ExtractLines([]byte("hello"))
	assert.ErrorIs(t, err, ErrNotPDF)
	_, _, err = ExtractLines([]byte("%PDF-1.4 garbage garbage"))
	assert.ErrorIs(t, err, ErrNotPDF)

	empty := renderPDF([]sampleLine{{"", 11, false}})
	_, _, err = ExtractLines(empty)
	assert.ErrorIs(t, err, ErrNoText)
}

func TestMultiPage(t *testing.T) {
	var lines []sampleLine
	for i := 1; i <= 40; i++ {
		lines = append(lines,
			sampleLine{Text: "" + itoa(i) + ". Question number " + itoa(i), Size: 11},
			sampleLine{Text: "A. yes", Size: 11}, sampleLine{Text: "B. no", Size: 11}, sampleLine{Text: "Answer: A", Size: 11})
	}
	out, pages, err := ExtractLines(renderPDF(lines))
	require.NoError(t, err)
	assert.Greater(t, pages, 1)
	assert.Len(t, Parse(out).Questions, 40)
}

func itoa(i int) string { return strconv.Itoa(i) }
