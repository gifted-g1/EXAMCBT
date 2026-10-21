// Command samplepdf writes the example question paper the PDF importer
// understands (the same file the dashboard's "Download example" serves).
//
//	go run ./scripts/samplepdf docs/sample-questions.pdf
package main

import (
	"fmt"
	"os"

	"examshield/internal/examimport"
)

func main() {
	out := "docs/sample-questions.pdf"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	if err := os.WriteFile(out, examimport.SamplePDF(), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("wrote", out)
}
