package examimport

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Question is one parsed question ready for preview. CorrectOption is nil
// when the paper didn't say which option is right: staff must pick it before
// importing an MCQ.
type Question struct {
	Number        int      `json:"number"`
	Kind          string   `json:"kind"` // MCQ | TEXT | GAP
	Prompt        string   `json:"prompt"`
	Options       []string `json:"options,omitempty"`
	CorrectOption *int     `json:"correct_option"`
	// GAP: accepted answers per blank, from "Answer: (i) TCP / T.C.P. (ii) 443".
	Blanks   [][]string `json:"blanks,omitempty"`
	Marks    float64    `json:"marks"`
	Warnings []string   `json:"warnings,omitempty"`
}

// Result is the preview returned to the dashboard.
type Result struct {
	Questions []Question `json:"questions"`
	Warnings  []string   `json:"warnings"`
	Pages     int        `json:"pages"`
	Lines     int        `json:"lines"`
}

var (
	// "1. ...", "1) ...", "Q1. ...", "Question 1: ...", "1 - ..."
	reQuestion = regexp.MustCompile(`(?i)^(?:q(?:uestion)?\s*)?(\d{1,3})\s*[.):\-]\s*(.+)$`)
	// "A. ...", "a) ...", "(b) ...", "C: ...", "D] ..."
	reOption = regexp.MustCompile(`^\(?([A-Ha-h])\s*[.):\]]\s*(.+)$`)
	// "Answer: B", "Ans - (c)", "Correct answer: D", "Key: a"
	reAnswer = regexp.MustCompile(`(?i)^(?:answer|ans|correct(?:\s+answer|\s+option)?|key)\s*[:=\-]\s*\(?([A-Ha-h])\)?\s*\.?\s*$`)
	// "Answer: <free text>" for written questions (model answer; not stored)
	reAnswerText = regexp.MustCompile(`(?i)^(?:answers?|ans|model\s+answer)\s*[:=\-]\s*(.+)$`)
	// "Marks: 2" / "Mark - 1.5"
	reMarksLine = regexp.MustCompile(`(?i)^marks?\s*[:=\-]\s*(\d+(?:\.\d+)?)\s*$`)
	// trailing "[2 marks]" / "(3 marks)" / "(1 mark)"
	reMarksInline = regexp.MustCompile(`(?i)\s*[\[(]\s*(\d+(?:\.\d+)?)\s*marks?\s*[\])]\s*$`)
	// "Type: TEXT" / "Type: essay" forces a written question
	reType = regexp.MustCompile(`(?i)^type\s*[:=\-]\s*(mcq|objective|text|essay|theory|written|gap|fill(?:[\s-]+in)?(?:[\s-]+the)?[\s-]+gaps?)\s*$`)
	// "Section 3: JavaScript", "PART B - Theory": ends the current question
	reSection = regexp.MustCompile(`(?i)^(?:section|part)\s+[A-Z0-9]{1,3}\b`)
	// generated page footer: "... Page 3 of 12"
	reFooter = regexp.MustCompile(`(?i)\bpage\s+\d{1,4}\s+of\s+\d{1,4}\s*$`)
	// "Answer Key" / "ANSWERS" section heading
	reKeyHeading = regexp.MustCompile(`(?i)^(?:answer\s*key|answers|marking\s+scheme)\s*:?\s*$`)
	// answer-key entries: "1. B", "1-B", "1) c", several per line allowed
	reKeyEntry = regexp.MustCompile(`(?i)(\d{1,3})\s*[.):\-]?\s*\(?([A-H])\)?(?:\s|,|;|$)`)
	// page furniture
	rePageNo = regexp.MustCompile(`(?i)^(?:page\s*)?\d{1,3}(?:\s*(?:of|/)\s*\d{1,3})?$`)
)

// Parse turns extracted lines into questions. Text before the first numbered
// question (title, instructions) is ignored.
func Parse(lines []string) Result {
	res := Result{Lines: len(lines), Questions: []Question{}, Warnings: []string{}}
	var cur *Question
	inKey := false
	gapRaw := "" // text of the current GAP answer line, which may wrap
	forcedText := map[int]bool{}

	finish := func() {
		if cur != nil {
			res.Questions = append(res.Questions, *cur)
			cur = nil
		}
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || rePageNo.MatchString(line) {
			continue
		}
		if reFooter.MatchString(line) {
			continue
		}
		if reSection.MatchString(line) {
			finish()
			inKey = false
			continue
		}
		if reKeyHeading.MatchString(line) {
			finish()
			inKey = true
			continue
		}
		if inKey {
			for _, m := range reKeyEntry.FindAllStringSubmatch(line, -1) {
				n, _ := strconv.Atoi(m[1])
				applyKey(&res, n, m[2])
			}
			continue
		}
		if cur != nil {
			if m := reAnswerText.FindStringSubmatch(line); m != nil && len(cur.Options) == 0 && blankCount(cur.Prompt) > 0 {
				gapRaw = m[1]
				cur.Blanks = splitGapAnswers(m[1], blankCount(cur.Prompt))
				continue
			}
			if m := reAnswer.FindStringSubmatch(line); m != nil {
				idx := letterIndex(m[1])
				cur.CorrectOption = &idx
				continue
			}
			if m := reMarksLine.FindStringSubmatch(line); m != nil {
				cur.Marks, _ = strconv.ParseFloat(m[1], 64)
				continue
			}
			if m := reType.FindStringSubmatch(line); m != nil {
				t := strings.ToLower(m[1])
				if strings.HasPrefix(t, "gap") || strings.HasPrefix(t, "fill") {
					continue // decided by the blanks in the prompt
				}
				if t != "mcq" && t != "objective" {
					forcedText[cur.Number] = true
				}
				continue
			}
			if opts := splitInlineOptions(line, len(cur.Options)); len(opts) > 1 {
				cur.Options = append(cur.Options, opts...)
				continue
			}
			if m := reOption.FindStringSubmatch(line); m != nil && letterIndex(m[1]) == len(cur.Options) {
				// Only accept the next letter in sequence (A, then B, ...), so a
				// sentence that happens to start with "I." or "a)" mid-prompt
				// isn't mistaken for an option.
				cur.Options = append(cur.Options, strings.TrimSpace(m[2]))
				continue
			}
			if reAnswerText.MatchString(line) && len(cur.Options) == 0 {
				continue // model answer for a written question: not stored
			}
		}
		if m := reQuestion.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[1])
			finish()
			cur = &Question{Number: n, Prompt: strings.TrimSpace(m[2]), Marks: 1}
			gapRaw = ""
			if mm := reMarksInline.FindStringSubmatch(cur.Prompt); mm != nil {
				cur.Marks, _ = strconv.ParseFloat(mm[1], 64)
				cur.Prompt = strings.TrimSpace(reMarksInline.ReplaceAllString(cur.Prompt, ""))
			}
			continue
		}
		if cur == nil {
			continue // title / instructions before question 1
		}
		if gapRaw != "" {
			// A long gap answer wrapped onto the next line.
			gapRaw += " " + line
			cur.Blanks = splitGapAnswers(gapRaw, blankCount(cur.Prompt))
			continue
		}
		// Continuation of the prompt or of the last option.
		if len(cur.Options) > 0 {
			cur.Options[len(cur.Options)-1] += " " + line
		} else {
			cur.Prompt += " " + line
		}
	}
	finish()

	seen := map[int]bool{}
	for i := range res.Questions {
		q := &res.Questions[i]
		if mm := reMarksInline.FindStringSubmatch(q.Prompt); mm != nil {
			q.Marks, _ = strconv.ParseFloat(mm[1], 64)
			q.Prompt = strings.TrimSpace(reMarksInline.ReplaceAllString(q.Prompt, ""))
		}
		switch {
		case !forcedText[q.Number] && len(q.Options) == 0 && blankCount(q.Prompt) > 0:
			q.Kind = "GAP"
			q.CorrectOption = nil
			if n := blankCount(q.Prompt); len(q.Blanks) != n {
				q.Warnings = append(q.Warnings, fmt.Sprintf("%d blank(s) but %d answer(s) found — add an answer line like \"Answer: (i) ... (ii) ...\"", n, len(q.Blanks)))
			}
		case forcedText[q.Number] || len(q.Options) == 0:
			q.Kind = "TEXT"
			if len(q.Options) > 0 {
				q.Prompt += "\n" + strings.Join(q.Options, "\n")
				q.Options = nil
			}
			q.Blanks = nil
			q.CorrectOption = nil
		case len(q.Options) == 1:
			q.Kind = "MCQ"
			q.Warnings = append(q.Warnings, "only one option found — add more options or change it to a written question")
		default:
			q.Kind = "MCQ"
		}
		if len(q.Options) > 8 {
			q.Warnings = append(q.Warnings, "more than 8 options; only A–H are supported")
		}
		if q.Kind == "MCQ" {
			if q.CorrectOption == nil {
				q.Warnings = append(q.Warnings, "no answer found — choose the correct option before importing")
			} else if *q.CorrectOption >= len(q.Options) {
				q.Warnings = append(q.Warnings, fmt.Sprintf("answer %s is not one of the options", string(rune('A'+*q.CorrectOption))))
				q.CorrectOption = nil
			}
		}
		if seen[q.Number] {
			q.Warnings = append(q.Warnings, fmt.Sprintf("question number %d appears more than once", q.Number))
		}
		seen[q.Number] = true
		if len(q.Prompt) > 10000 {
			q.Prompt = q.Prompt[:10000]
			q.Warnings = append(q.Warnings, "prompt truncated to 10,000 characters")
		}
	}
	if len(res.Questions) == 0 {
		res.Warnings = append(res.Warnings, "no numbered questions found — number each question like \"1.\" and each option like \"A.\" (see the example PDF)")
	}
	return res
}

func applyKey(res *Result, n int, letter string) {
	for i := range res.Questions {
		if res.Questions[i].Number == n {
			idx := letterIndex(letter)
			res.Questions[i].CorrectOption = &idx
			return
		}
	}
	res.Warnings = append(res.Warnings, fmt.Sprintf("answer key lists question %d, which was not found", n))
}

func letterIndex(s string) int {
	return int(strings.ToUpper(s)[0] - 'A')
}

// reInlineOpt finds option markers inside a line: "A. 3   B. 4   C) 5".
var reInlineOpt = regexp.MustCompile(`(?:^|\s)\(?([A-H])\s*[.)]\s+`)

// splitInlineOptions splits "A. 3  B. 4  C. 5" into ["3","4","5"] when the
// markers are consecutive letters starting at the next expected option.
// It returns nil for anything else.
func splitInlineOptions(line string, next int) []string {
	locs := reInlineOpt.FindAllStringSubmatchIndex(line, -1)
	if len(locs) < 2 || locs[0][0] != 0 {
		return nil
	}
	var out []string
	for i, loc := range locs {
		if letterIndex(line[loc[2]:loc[3]]) != next+i {
			return nil
		}
		end := len(line)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		text := strings.TrimSpace(line[loc[1]:end])
		if text == "" {
			return nil
		}
		out = append(out, text)
	}
	return out
}

var (
	reBlank = regexp.MustCompile(`_{3,}`)
	// "(i)", "(ii)", "(1)", "(a)", "i)", "1." markers between gap answers
	reGapMarker = regexp.MustCompile(`(?i)(?:^|\s|;|,)\(?(?:[ivx]{1,4}|\d{1,2}|[a-j])\)\s*`)
)

func blankCount(prompt string) int { return len(reBlank.FindAllStringIndex(prompt, -1)) }

// splitGapAnswers turns "(i) TCP / Transmission Control Protocol (ii) 443"
// or "TCP | 443" into one list of accepted answers per blank. Alternatives
// for the same blank are separated by " / ".
func splitGapAnswers(s string, n int) [][]string {
	var parts []string
	switch {
	case strings.Contains(s, "|"):
		parts = strings.Split(s, "|")
	case n > 1 && len(reGapMarker.FindAllStringIndex(s, -1)) >= 2:
		parts = reGapMarker.Split(s, -1)
	default:
		parts = []string{s}
	}
	var out [][]string
	for _, p := range parts {
		p = strings.Trim(strings.TrimSpace(p), ";,")
		if p == "" {
			continue
		}
		var alts []string
		for _, a := range strings.Split(p, " / ") {
			if a = strings.TrimSpace(a); a != "" {
				alts = append(alts, a)
			}
		}
		out = append(out, alts)
	}
	return out
}
