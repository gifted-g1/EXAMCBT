package questionbank

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"strings"

	"examshield/internal/examimport"
)

// Item is one question in its final, numbered, shuffled form.
type Item struct {
	Number  int
	Kind    string // MCQ | GAP | TEXT
	Prompt  string
	Options []string
	Answer  int        // index into Options (MCQ only)
	Blanks  [][]string // accepted answers per blank (GAP only)
	Marks   float64
}

// Items numbers the questions of the given subjects from 1 and shuffles each
// MCQ's options with a seed derived from the question text, so the answer
// letters are well spread but identical every time the PDFs are rebuilt.
func Items(subjects []Subject) [][]Item {
	n := 0
	out := make([][]Item, len(subjects))
	for si, s := range subjects {
		for _, q := range s.MCQs {
			n++
			opts, ans := shuffle(s.Slug+"|"+q.Q, q.Options, q.Answer)
			out[si] = append(out[si], Item{Number: n, Kind: "MCQ", Prompt: q.Q, Options: opts, Answer: ans, Marks: 1})
		}
		for _, g := range s.Gaps {
			n++
			out[si] = append(out[si], Item{Number: n, Kind: "GAP", Prompt: g.Q, Blanks: g.Blanks, Marks: float64(len(g.Blanks))})
		}
		for _, w := range s.Written {
			n++
			out[si] = append(out[si], Item{Number: n, Kind: "TEXT", Prompt: w.Q, Marks: w.Marks})
		}
	}
	return out
}

func shuffle(seedText string, options []string, answer int) ([]string, int) {
	h := fnv.New64a()
	h.Write([]byte(seedText))
	r := rand.New(rand.NewSource(int64(h.Sum64())))
	idx := r.Perm(len(options))
	out := make([]string, len(options))
	newAnswer := 0
	for to, from := range idx {
		out[to] = options[from]
		if from == answer {
			newAnswer = to
		}
	}
	return out, newAnswer
}

// Options for Render.
type Options struct {
	Title    string
	Subtitle string
	Answers  bool // include "Answer:" lines (needed for import; omit for a student paper)
}

// Render lays out subjects as a PDF the importer can read back exactly.
func Render(subjects []Subject, opt Options) []byte {
	items := Items(subjects)
	total, mcq, gaps, written, marks := 0, 0, 0, 0, 0.0
	for _, its := range items {
		for _, it := range its {
			total++
			marks += it.Marks
			switch it.Kind {
			case "MCQ":
				mcq++
			case "GAP":
				gaps++
			default:
				written++
			}
		}
	}

	var L []examimport.Line
	add := func(l examimport.Line) { L = append(L, l) }

	// Title page (everything before question 1 is ignored by the importer).
	add(examimport.Line{Text: "EXAM SHIELD", Size: 12, Bold: true, Gap: 120})
	add(examimport.Line{Text: opt.Title, Size: 26, Bold: true, Gap: 6})
	if opt.Subtitle != "" {
		add(examimport.Line{Text: opt.Subtitle, Size: 13, Gap: 4})
	}
	add(examimport.Line{Text: fmt.Sprintf("%d questions: %d multiple choice, %d fill in the gap and %d written. Total %s marks.", total, mcq, gaps, written, fmtMarks(marks)), Size: 11, Gap: 18})
	if opt.Answers {
		add(examimport.Line{Text: "Edition: with answers (staff copy, ready to import into Exam Shield).", Size: 11})
	} else {
		add(examimport.Line{Text: "Edition: question paper (no answers shown).", Size: 11})
	}
	if len(subjects) > 1 {
		add(examimport.Line{Text: "Contents", Size: 14, Bold: true, Gap: 28, PageBreak: len(subjects) > 12})
		for si, s := range subjects {
			first, last := items[si][0].Number, items[si][len(items[si])-1].Number
			add(examimport.Line{Text: fmt.Sprintf("Section %d: %s  -  questions %d to %d", si+1, s.Name, first, last), Size: 11, Indent: 12})
		}
	}
	add(examimport.Line{Text: "How this paper is laid out", Size: 14, Bold: true, Gap: 28})
	for _, p := range []string{
		"Questions are numbered in one sequence. Multiple-choice options are lettered with a full stop; each multiple-choice question is worth one mark unless stated.",
		"Fill-in-the-gap questions show each blank as a line; they earn one mark per blank, shown in square brackets, and are marked automatically.",
		"Written questions show their marks in square brackets and are marked by an examiner.",
		"To use this paper in Exam Shield, open an exam, choose Questions and then Import from PDF, and upload this file. Review the preview and import.",
	} {
		add(examimport.Line{Text: p, Size: 10.5, Indent: 12, Gap: 2})
	}

	for si, s := range subjects {
		add(examimport.Line{Text: fmt.Sprintf("Section %d: %s", si+1, s.Name), Size: 17, Bold: true, PageBreak: true})
		add(examimport.Line{Text: s.Blurb, Size: 10.5, Gap: 2})
		for _, it := range items[si] {
			prompt := it.Prompt
			if it.Kind != "MCQ" {
				prompt += " " + marksTag(it.Marks)
			}
			keep := 0
			if it.Kind == "MCQ" {
				keep = len(it.Options)
				if opt.Answers {
					keep++
				}
			} else if it.Kind == "GAP" && opt.Answers {
				keep = 1
			}
			add(examimport.Line{Text: fmt.Sprintf("%d. %s", it.Number, prompt), Size: 11, Bold: true, Gap: 10, KeepNext: keep})
			if it.Kind == "MCQ" {
				for oi, o := range it.Options {
					add(examimport.Line{Text: fmt.Sprintf("%c. %s", 'A'+oi, o), Size: 11, Indent: 18})
				}
				if opt.Answers {
					add(examimport.Line{Text: fmt.Sprintf("Answer: %c", 'A'+it.Answer), Size: 10, Indent: 18, Gap: 2})
				}
			}
			if it.Kind == "GAP" && opt.Answers {
				alts := make([]string, len(it.Blanks))
				for i, a := range it.Blanks {
					alts[i] = strings.Join(a, " / ")
				}
				add(examimport.Line{Text: "Answer: " + strings.Join(alts, " | "), Size: 10, Indent: 18, Gap: 2})
			}
		}
	}
	footer := "Exam Shield - " + opt.Title
	return examimport.RenderPDF(L, examimport.RenderOptions{Footer: footer})
}

// marksTag renders "[1 mark]" or "[4 marks]".
func marksTag(m float64) string {
	if m == 1 {
		return "[1 mark]"
	}
	return "[" + fmtMarks(m) + " marks]"
}

func fmtMarks(m float64) string {
	s := fmt.Sprintf("%.1f", m)
	return strings.TrimSuffix(s, ".0")
}
