// Command questionbank writes the Exam Shield question bank PDFs.
//
//	go run ./scripts/questionbank [outdir]      (default docs/question-bank)
//
// Output:
//
//	exam-shield-question-bank.pdf   all 19 subjects, with answers (import this)
//	exam-shield-question-paper.pdf  all 19 subjects, no answers (to print/present)
//	subjects/<subject>.pdf          one subject each, with answers
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"examshield/internal/questionbank"
)

func main() {
	dir := "docs/question-bank"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	must(os.MkdirAll(filepath.Join(dir, "subjects"), 0o755))
	write := func(path string, data []byte) {
		must(os.WriteFile(path, data, 0o644))
		fmt.Printf("wrote %-60s %6.1f KB\n", path, float64(len(data))/1024)
	}
	sub := "Computing question bank: 19 subjects across computing, with multiple-choice, fill-in-the-gap and written questions"
	write(filepath.Join(dir, "exam-shield-question-bank.pdf"),
		questionbank.Render(questionbank.Subjects, questionbank.Options{Title: "Question Bank", Subtitle: sub, Answers: true}))
	write(filepath.Join(dir, "exam-shield-question-paper.pdf"),
		questionbank.Render(questionbank.Subjects, questionbank.Options{Title: "Question Paper", Subtitle: sub, Answers: false}))
	for _, s := range questionbank.Subjects {
		write(filepath.Join(dir, "subjects", s.Slug+".pdf"),
			questionbank.Render([]questionbank.Subject{s}, questionbank.Options{Title: s.Name + " Question Bank", Subtitle: s.Blurb, Answers: true}))
	}
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
