package examimport

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// Line is one paragraph of a generated PDF. Long text is word-wrapped.
type Line struct {
	Text      string
	Size      float64 // points; 0 = 11
	Bold      bool
	Indent    float64 // extra left margin in points
	PageBreak bool    // start a new page before this line
	Gap       float64 // extra space above, in points
	KeepNext  int     // keep this line and the next KeepNext lines on one page
}

// RenderOptions controls RenderPDF.
type RenderOptions struct {
	// Footer, when non-empty, is printed at the bottom of every page followed
	// by "Page N of M" (the importer ignores both).
	Footer string
}

const (
	pageW, pageH = 595.0, 842.0 // A4
	marginX      = 56.0
	marginTop    = 60.0
	marginBottom = 64.0
)

// riskyStart matches text the importer would treat as a new question or
// option. A wrapped continuation line must never begin like this.
var riskyStart = regexp.MustCompile(`^(?:\(?[A-Ha-h]\s*[.):\]]|(?:q(?:uestion)?\s*)?\d{1,3}\s*[.):\-]|(?i:answer|ans|marks?|type|section|part)\b)`)

// RenderPDF writes a text-only PDF 1.4 (A4, Helvetica / Helvetica-Bold,
// WinAnsi). Non-ASCII characters are replaced, so keep text ASCII.
func RenderPDF(lines []Line, opt RenderOptions) []byte {
	type placed struct {
		text       string
		size, x, y float64
		bold       bool
	}
	var pages [][]placed
	var cur []placed
	y := pageH - marginTop
	newPage := func() {
		pages = append(pages, cur)
		cur = nil
		y = pageH - marginTop
	}
	for li, l := range lines {
		size := l.Size
		if size == 0 {
			size = 11
		}
		if l.PageBreak && (len(cur) > 0 || len(pages) > 0) {
			newPage()
		}
		lead := size * 1.45
		y -= l.Gap
		if l.KeepNext > 0 && len(cur) > 0 {
			if need := blockHeight(lines[li:min(len(lines), li+l.KeepNext+1)]); y-need < marginBottom {
				newPage()
			}
		}
		if l.Text == "" {
			y -= lead
			continue
		}
		width := pageW - 2*marginX - l.Indent
		for _, seg := range wrap(l.Text, size, width, l.Bold) {
			if y-lead < marginBottom {
				newPage()
			}
			y -= lead
			x := marginX + l.Indent
			cur = append(cur, placed{seg, size, x, y, l.Bold})
		}
	}
	pages = append(pages, cur)
	if len(pages) > 1 && len(pages[len(pages)-1]) == 0 {
		pages = pages[:len(pages)-1]
	}

	contents := make([]string, len(pages))
	for i, p := range pages {
		var b strings.Builder
		for _, t := range p {
			font := "F1"
			if t.bold {
				font = "F2"
			}
			fmt.Fprintf(&b, "BT /%s %.1f Tf %.1f %.1f Td (%s) Tj ET\n", font, t.size, t.x, t.y, pdfEscape(t.text))
		}
		if opt.Footer != "" {
			fmt.Fprintf(&b, "0.6 g\nBT /F1 8.0 Tf %.1f 34.0 Td (%s) Tj ET\n", marginX, pdfEscape(opt.Footer))
			fmt.Fprintf(&b, "BT /F1 8.0 Tf %.1f 34.0 Td (Page %d of %d) Tj ET\n0 g\n", pageW-marginX-60, i+1, len(pages))
		}
		contents[i] = b.String()
	}
	return assemble(contents)
}

// wrap splits text into lines that fit width, never starting a continuation
// with something the importer would misread (see riskyStart).
func wrap(text string, size, width float64, bold bool) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var out []string
	line := words[0]
	for _, w := range words[1:] {
		if textWidth(line+" "+w, size, bold) <= width {
			line += " " + w
			continue
		}
		out = append(out, line)
		line = w
	}
	out = append(out, line)
	// Repair continuation lines that would look like a new question/option by
	// pulling their first word back onto the previous line.
	for i := 1; i < len(out); i++ {
		for riskyStart.MatchString(out[i]) {
			first, rest, _ := strings.Cut(out[i], " ")
			out[i-1] += " " + first
			if rest == "" {
				out = append(out[:i], out[i+1:]...)
				i--
				break
			}
			out[i] = rest
		}
	}
	return out
}

// textWidth approximates Helvetica advance widths (units per 1000 em).
func textWidth(s string, size float64, bold bool) float64 {
	var units float64
	for _, r := range s {
		switch {
		case r == ' ':
			units += 278
		case strings.ContainsRune("iljI.,:;'!|", r):
			units += 250
		case strings.ContainsRune("ftr()[]-/", r):
			units += 333
		case r >= 'A' && r <= 'Z', strings.ContainsRune("mwMW@%", r):
			units += 722
		case r >= '0' && r <= '9':
			units += 556
		default:
			units += 556
		}
	}
	if bold {
		units *= 1.06
	}
	return units * size / 1000
}

func assemble(contents []string) []byte {
	var buf bytes.Buffer
	offsets := []int{}
	obj := func(body string) {
		offsets = append(offsets, buf.Len())
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", len(offsets), body)
	}
	buf.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	kids := make([]string, len(contents))
	for i := range contents {
		kids[i] = fmt.Sprintf("%d 0 R", 5+2*i)
	}
	obj("<< /Type /Catalog /Pages 2 0 R >>")
	obj(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(contents)))
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>")
	for i, content := range contents {
		obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Resources << /Font << /F1 3 0 R /F2 4 0 R >> >> /Contents %d 0 R >>",
			pageW, pageH, 6+2*i))
		obj(fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content))
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, o := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xref)
	return buf.Bytes()
}

func pdfEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\\' || r == '(' || r == ')':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 32 || r > 126:
			b.WriteByte('?') // keep generated text ASCII
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// blockHeight is the vertical space a run of lines needs, including wraps.
func blockHeight(lines []Line) float64 {
	var h float64
	for i, l := range lines {
		size := l.Size
		if size == 0 {
			size = 11
		}
		if i > 0 {
			h += l.Gap
		}
		n := len(wrap(l.Text, size, pageW-2*marginX-l.Indent, l.Bold))
		if n == 0 {
			n = 1
		}
		h += float64(n) * size * 1.45
	}
	return h
}
