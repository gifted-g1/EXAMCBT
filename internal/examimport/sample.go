package examimport

// SampleLines is the example question paper staff can download. It shows
// every format the importer understands.
var SampleLines = []sampleLine{
	{"IFT 211 - Computer Networks: Mid-Semester Test", 16, true},
	{"Instructions: Answer ALL questions. This header text is ignored by the importer.", 10, false},
	{"", 10, false},
	{"1. Which layer of the OSI model is responsible for routing packets?", 11, false},
	{"A. Data link", 11, false},
	{"B. Network", 11, false},
	{"C. Transport", 11, false},
	{"D. Session", 11, false},
	{"Answer: B", 11, false},
	{"", 11, false},
	{"2. What does CIDR stand for? [2 marks]", 11, false},
	{"A. Classless Inter-Domain Routing", 11, false},
	{"B. Common Internet Data Relay", 11, false},
	{"C. Central IP Distribution Registry", 11, false},
	{"D. Class Independent Domain Routing", 11, false},
	{"Answer: A", 11, false},
	{"", 11, false},
	{"3. How many usable host addresses are in a /24 IPv4 subnet?", 11, false},
	{"A. 256    B. 255    C. 254    D. 128", 11, false},
	{"Answer: C", 11, false},
	{"", 11, false},
	{"4. Which HTTP status code means the resource is locked?", 11, false},
	{"(a) 403", 11, false},
	{"(b) 409", 11, false},
	{"(c) 423", 11, false},
	{"(d) 451", 11, false},
	{"Marks: 1", 11, false},
	{"", 11, false},
	{"5. Which device forwards frames using MAC addresses?", 11, false},
	{"A) Router", 11, false},
	{"B) Switch", 11, false},
	{"C) Hub", 11, false},
	{"D) Modem", 11, false},
	{"", 11, false},
	{"6. In two sentences, explain why an exam server should only listen on the", 11, false},
	{"LAN interface. (5 marks)", 11, false},
	{"Type: TEXT", 11, false},
	{"", 11, false},
	{"Answer Key", 13, true},
	{"5. B", 11, false},
	{"", 10, false},
	{"Format guide: number questions 1. 2. 3.; letter options A. B. C. (or a) / (a));", 9, false},
	{"give the answer as 'Answer: B' under the question or in an 'Answer Key' section;", 9, false},
	{"marks as 'Marks: 2' or '[2 marks]' (default 1); questions without options are written answers.", 9, false},
}

type sampleLine struct {
	Text string
	Size float64
	Bold bool
}

// SamplePDF renders SampleLines as a small, valid PDF (A4, Helvetica).
func SamplePDF() []byte {
	return renderPDF(SampleLines)
}

// renderPDF renders sample lines with the shared writer.
func renderPDF(lines []sampleLine) []byte {
	out := make([]Line, len(lines))
	for i, l := range lines {
		out[i] = Line{Text: l.Text, Size: l.Size, Bold: l.Bold}
	}
	return RenderPDF(out, RenderOptions{})
}
