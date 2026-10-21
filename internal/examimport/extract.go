// Package examimport turns a question-paper PDF into exam questions.
//
// Extraction rebuilds text lines from glyph positions, then Parse recognises
// numbered questions, lettered options, answers and marks. Nothing is saved
// here: the API returns a preview that staff confirm before import.
package examimport

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/ledongthuc/pdf"
)

const (
	MaxPDFBytes = 20 << 20
	MaxPages    = 200
)

var (
	ErrNotPDF    = errors.New("the file is not a readable PDF")
	ErrEncrypted = errors.New("the PDF is password-protected; remove the password and try again")
	ErrNoText    = errors.New("no text found in the PDF — it looks like a scanned image; export the paper from Word/Google Docs as PDF instead, or run OCR first")
	ErrTooLarge  = fmt.Errorf("PDF is larger than %d MB or %d pages", MaxPDFBytes>>20, MaxPages)
)

// ExtractLines returns the document's text as lines in reading order
// (top-to-bottom, left-to-right, page by page).
func ExtractLines(data []byte) (lines []string, pages int, err error) {
	if len(data) > MaxPDFBytes {
		return nil, 0, ErrTooLarge
	}
	if !bytes.HasPrefix(bytes.TrimLeft(data, "\x00\t\r\n "), []byte("%PDF-")) {
		return nil, 0, ErrNotPDF
	}
	// The PDF library panics on some malformed files; never let that take
	// the server down.
	defer func() {
		if r := recover(); r != nil {
			lines, pages, err = nil, 0, ErrNotPDF
		}
	}()
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "encrypt") || strings.Contains(err.Error(), "password") {
			return nil, 0, ErrEncrypted
		}
		return nil, 0, ErrNotPDF
	}
	pages = r.NumPage()
	if pages > MaxPages {
		return nil, pages, ErrTooLarge
	}
	for i := 1; i <= pages; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		lines = append(lines, pageLines(p.Content().Text)...)
	}
	nonEmpty := 0
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			nonEmpty++
		}
	}
	if nonEmpty == 0 {
		return nil, pages, ErrNoText
	}
	return lines, pages, nil
}

// pageLines groups glyphs into lines by baseline and joins them, inserting
// a space wherever the horizontal gap is wider than a fraction of the font
// size (many PDF producers position words individually without spaces).
func pageLines(texts []pdf.Text) []string {
	if len(texts) == 0 {
		return nil
	}
	ts := make([]pdf.Text, 0, len(texts))
	for _, t := range texts {
		if t.S != "" {
			ts = append(ts, t)
		}
	}
	// Sort top-to-bottom (PDF Y grows upwards), then left-to-right.
	sort.SliceStable(ts, func(i, j int) bool {
		if math.Abs(ts[i].Y-ts[j].Y) > lineTolerance(ts[i], ts[j]) {
			return ts[i].Y > ts[j].Y
		}
		return ts[i].X < ts[j].X
	})

	var lines []string
	var cur strings.Builder
	var prev *pdf.Text
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			lines = append(lines, collapseSpaces(s))
		}
		cur.Reset()
	}
	for i := range ts {
		t := &ts[i]
		if prev != nil && math.Abs(prev.Y-t.Y) > lineTolerance(*prev, *t) {
			flush()
			prev = nil
		}
		if prev != nil {
			end := prev.X + prev.W
			if prev.W <= 0 {
				end = prev.X + 0.5*fontSize(*prev)*float64(len([]rune(prev.S)))
			}
			if t.X-end > 0.2*fontSize(*t) && !strings.HasSuffix(cur.String(), " ") && !strings.HasPrefix(t.S, " ") {
				cur.WriteByte(' ')
			}
		}
		cur.WriteString(t.S)
		prev = t
	}
	flush()
	return lines
}

func fontSize(t pdf.Text) float64 {
	if t.FontSize > 0 {
		return t.FontSize
	}
	return 10
}

func lineTolerance(a, b pdf.Text) float64 {
	return 0.35 * math.Max(fontSize(a), fontSize(b))
}

func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
