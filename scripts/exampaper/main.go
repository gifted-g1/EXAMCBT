// Command exampaper builds the Faculty of Computing Comprehensive
// Examination from one source of questions:
//
//	go run ./scripts/exampaper [outdir]      (default docs/exam-paper)
//
// Output:
//
//	computing-comprehensive-exam.html        print layout (no answers); made into the PDF by print.mjs
//	computing-comprehensive-exam-import.pdf  staff copy with gap answers, for Questions > Import from PDF
//
// The command checks the paper's shape (30 questions, 12/10/8 per section,
// three per subject area, 40/20/40 = 100 marks) and that the import copy
// parses back exactly, and fails otherwise.
package main

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"examshield/internal/examimport"
)

var areas = []string{
	"Computer Science",
	"Software Engineering",
	"Information Technology and Information Systems",
	"Computer Networks and Telecommunications",
	"Cloud Computing, DevOps and SRE",
	"Cybersecurity",
	"Artificial Intelligence and Machine Learning",
	"Data Science, Databases and Data Engineering",
	"Computer Engineering and Embedded Systems",
	"Distributed Systems and Emerging Technologies",
}

// Q is one question. Blanks ("______") are only used in Section B; Blanks
// holds the accepted answers per blank for the staff import copy.
type Q struct {
	Area   int // index into areas
	Marks  int
	Prompt string
	Code   string   // optional monospace block
	Parts  []string // optional (a), (b), (c) sub-questions
	Blanks [][]string
	Lines  int // ruled answer lines on the printed paper
}

type Section struct {
	Letter, Title, Instructions string
	Questions                   []Q
}

const blank = "______"

var sections = []Section{
	{"A", "Theory and Analytical Questions",
		"Answer ALL questions in this section. Write concise, well-reasoned answers in the space provided. Where a question is open to more than one defensible answer, state your assumptions.",
		[]Q{
			{Area: 0, Marks: 3, Lines: 9, Prompt: "A dynamic array doubles its capacity whenever it is full. Using amortised analysis, show that the cost of appending n elements is O(n) in total, so each append is O(1) amortised, even though a single append can cost O(n). Explain one practical reason some libraries grow by a factor of 1.5 instead of 2."},
			{Area: 1, Marks: 4, Lines: 11, Prompt: "A 12-person engineering team maintains a single e-commerce application and proposes to split it into about 20 microservices. Compare this proposal with restructuring it as a modular monolith, considering deployment independence, data consistency, operational overhead and Conway's law. State your assumptions and recommend one option."},
			{Area: 2, Marks: 3, Lines: 9, Prompt: "Distinguish between IT governance and IT management. Explain how a governance framework such as COBIT helps an organisation align IT investment with business objectives and measure the value delivered."},
			{Area: 2, Marks: 4, Lines: 11, Prompt: "A university plans to replace its 15-year-old on-premises student records system with a cloud-based ERP platform within one academic year. Identify three significant risks of this digital transformation (technical, organisational or contractual) and propose a specific mitigation for each."},
			{Area: 3, Marks: 3, Lines: 9, Prompt: "Compare link-state routing (for example OSPF) with distance-vector routing (for example RIP) in terms of convergence speed, scalability and resource use. Explain the count-to-infinity problem and how split horizon with poison reverse reduces it."},
			{Area: 4, Marks: 3, Lines: 9, Prompt: "Define a Service Level Indicator (SLI), a Service Level Objective (SLO) and an error budget. Explain how an error budget policy can be used to balance release velocity against reliability, with a numerical example for a 99.9% monthly availability SLO."},
			{Area: 5, Marks: 3, Lines: 9, Prompt: "Explain why passwords should be stored using a slow, salted, memory-hard function such as Argon2id or bcrypt rather than a single round of SHA-256. Discuss the roles of the salt and a pepper, and the economics of an offline attack after a database breach."},
			{Area: 6, Marks: 4, Lines: 11, Prompt: "A bank's fraud-detection model reports 99% accuracy on a test set in which only 0.5% of transactions are fraudulent. Explain why accuracy is misleading here. Recommend more suitable evaluation metrics and describe how you would choose the decision threshold when a missed fraud costs far more than a false alarm."},
			{Area: 7, Marks: 3, Lines: 9, Prompt: "Describe the dirty read, non-repeatable read and phantom read anomalies, and state which ANSI SQL isolation levels prevent each. Explain, with an example, why snapshot isolation still permits write skew."},
			{Area: 8, Marks: 3, Lines: 9, Prompt: "Explain how a virtual address is translated to a physical address in a paged virtual-memory system with a Translation Lookaside Buffer (TLB). Show how the TLB hit ratio and page-fault rate affect the effective memory access time."},
			{Area: 8, Marks: 4, Lines: 11, Prompt: "A battery-powered IoT soil sensor must report a reading every 10 minutes over an unreliable wireless link for at least two years. Compare a polling design with an interrupt-driven, deep-sleep design, and compare MQTT with HTTP for this device, in terms of energy use, reliability and security."},
			{Area: 9, Marks: 3, Lines: 9, Prompt: "State the CAP theorem and explain how the PACELC formulation extends it. For a globally replicated online shopping-cart service, justify whether you would favour availability or consistency during a network partition, and what you would favour in normal operation."},
		}},
	{"B", "Fill in the Gaps",
		"Answer ALL questions in this section. Write the word, number or short phrase that correctly completes each numbered blank in the space provided below the statement. Each question carries 2 marks.",
		[]Q{
			{Area: 0, Marks: 2, Prompt: "Binary search on a sorted array of n elements has a worst-case time complexity of (i) " + blank + ", whereas building a binary heap bottom-up from n unsorted elements takes (ii) " + blank + " time.",
				Blanks: [][]string{{"O(log n)", "log n", "logarithmic"}, {"O(n)", "linear", "n"}}},
			{Area: 1, Marks: 2, Prompt: "Among the SOLID principles, the (i) " + blank + " Principle states that a class should have only one reason to change, and the (ii) " + blank + " Principle states that software entities should be open for extension but closed for modification.",
				Blanks: [][]string{{"Single Responsibility", "Single-Responsibility", "SRP"}, {"Open/Closed", "Open-Closed", "Open Closed", "OCP"}}},
			{Area: 2, Marks: 2, Prompt: "In ITIL, the practice that restores normal service as quickly as possible after an unplanned interruption is (i) " + blank + " management, while the practice that identifies and removes the underlying cause of recurring incidents is (ii) " + blank + " management.",
				Blanks: [][]string{{"incident"}, {"problem"}}},
			{Area: 3, Marks: 2, Prompt: "TCP establishes a connection with a three-way handshake of SYN, (i) " + blank + " and ACK segments. An IPv4 subnet with prefix length /26 provides (ii) " + blank + " usable host addresses.",
				Blanks: [][]string{{"SYN-ACK", "SYN ACK", "SYN/ACK"}, {"62", "sixty-two", "sixty two"}}},
			{Area: 4, Marks: 2, Prompt: "In Kubernetes, the smallest deployable unit is the (i) " + blank + ". When a (ii) " + blank + " probe fails, the kubelet restarts the container, whereas when a (iii) " + blank + " probe fails, the pod is removed from Service endpoints without being restarted.",
				Blanks: [][]string{{"Pod"}, {"liveness"}, {"readiness"}}},
			{Area: 5, Marks: 2, Prompt: "To send a confidential message with public-key cryptography, the sender encrypts it with the recipient's (i) " + blank + " key, and a digital signature is created with the signer's (ii) " + blank + " key. A single-page web application using OAuth 2.0 should use the authorization code flow with (iii) " + blank + ".",
				Blanks: [][]string{{"public"}, {"private"}, {"PKCE", "Proof Key for Code Exchange"}}},
			{Area: 6, Marks: 2, Prompt: "In gradient descent, the hyperparameter that scales the size of each parameter update is the (i) " + blank + ". A model that performs well on its training data but poorly on unseen data is said to be (ii) " + blank + ".",
				Blanks: [][]string{{"learning rate", "step size"}, {"overfitting", "overfitted", "overfit", "over-fitting"}}},
			{Area: 7, Marks: 2, Prompt: "The ACID property which guarantees that the effects of a committed transaction survive a system crash is (i) " + blank + ". In SQL, the clause that filters groups after aggregation is (ii) " + blank + ".",
				Blanks: [][]string{{"durability"}, {"HAVING"}}},
			{Area: 8, Marks: 2, Prompt: "In a pipelined CPU, a stall caused by an instruction needing the result of an earlier instruction that has not yet been written back is a (i) " + blank + " hazard. It is commonly reduced by a hardware technique called (ii) " + blank + ", also known as bypassing.",
				Blanks: [][]string{{"data", "RAW", "read-after-write"}, {"forwarding", "operand forwarding", "data forwarding"}}},
			{Area: 9, Marks: 2, Prompt: "A Raft cluster of five servers can continue to commit entries with at most (i) " + blank + " servers failed. In a blockchain, each block stores the cryptographic (ii) " + blank + " of the previous block, which makes past blocks tamper-evident.",
				Blanks: [][]string{{"2", "two"}, {"hash", "hash value", "digest"}}},
		}},
	{"C", "Applied Technical Problems",
		"Answer ALL questions in this section. Each question carries 5 marks. Show your reasoning; marks are awarded for correct analysis and justified decisions, not only final answers.",
		[]Q{
			{Area: 0, Marks: 5, Lines: 13, Prompt: "Consider the following function:",
				Code: "function fib(n):\n    if n < 2: return n\n    return fib(n - 1) + fib(n - 2)",
				Parts: []string{
					"State its time complexity and explain why. (1 mark)",
					"Write pseudocode for a version that runs in O(n) time and O(1) extra space. (3 marks)",
					"Name one technique that computes fib(n) in O(log n) arithmetic operations. (1 mark)",
				}},
			{Area: 1, Marks: 5, Lines: 13, Prompt: "An integration test passes on developers' machines but fails in about 10% of CI runs. The test creates an order, calls an endpoint that publishes an event asynchronously, and then immediately asserts that a notification row exists in a database shared by all CI jobs.",
				Parts: []string{
					"Identify two likely causes of the flakiness. (2 marks)",
					"Describe how you would confirm each cause. (1 mark)",
					"Propose fixes to the test and the test environment that make it deterministic. (2 marks)",
				}},
			{Area: 3, Marks: 5, Lines: 13, Prompt: "Staff at a branch office connected over a site-to-site VPN report two problems: (1) they can ping 8.8.8.8 but cannot open any website by name; (2) after the first problem is fixed, small web pages load but large downloads and some HTTPS sites stall.",
				Parts: []string{
					"Explain a systematic diagnosis of problem (1), naming the tools or commands you would use. (2 marks)",
					"Explain the most likely cause of problem (2) and two ways to fix it. (3 marks)",
				}},
			{Area: 4, Marks: 5, Lines: 13, Prompt: "A team deploys a stateless web service to production by running a single manual command from a laptop. Each release causes about two minutes of downtime, and a faulty release last month took an hour to roll back. Design a replacement delivery process.",
				Parts: []string{
					"Outline the CI/CD pipeline stages, including how infrastructure is managed as code and how its state is stored safely. (2 marks)",
					"Choose blue-green or canary deployment and justify the choice. (1 mark)",
					"Explain how releases are rolled back automatically using monitoring signals. (2 marks)",
				}},
			{Area: 5, Marks: 5, Lines: 13, Prompt: "A login handler contains the following code:",
				Code: "query = \"SELECT * FROM users WHERE email = '\" + email +\n        \"' AND password = '\" + password + \"'\"\nrow = db.execute(query)\nif row: create_session(row.id)",
				Parts: []string{
					"Identify two security weaknesses and explain how each could be exploited. (2 marks)",
					"Rewrite the logic in pseudocode to remove both weaknesses. (2 marks)",
					"Name one additional control that limits online password-guessing attacks. (1 mark)",
				}},
			{Area: 6, Marks: 5, Lines: 13, Prompt: "A machine-learning model used to pre-screen loan applications approves applicants from one demographic group at a markedly lower rate than others, and its precision has fallen steadily in the six months since deployment.",
				Parts: []string{
					"Explain two fairness metrics you would compute and what each would reveal. (2 marks)",
					"Explain how you would detect and respond to data drift and concept drift. (2 marks)",
					"Describe one human-oversight mechanism appropriate for this decision. (1 mark)",
				}},
			{Area: 7, Marks: 5, Lines: 13, Prompt: "A retailer has the tables Customers(customer_id, name, region) and Orders(order_id, customer_id, order_date, amount).",
				Parts: []string{
					"Write an SQL query returning each region's total order amount for 2025, only for regions whose total exceeds 100000, highest first. (3 marks)",
					"Propose an index that would speed up the query and explain why. (1 mark)",
					"The table is loaded by a nightly pipeline that sometimes reruns after failures. Explain how to make the load idempotent. (1 mark)",
				}},
			{Area: 9, Marks: 5, Lines: 13, Prompt: "A payment service consumes charge requests from a message queue that provides at-least-once delivery. Occasionally a customer is charged twice for one purchase.",
				Parts: []string{
					"Explain how duplicate charges arise in this design. (1 mark)",
					"Design an idempotency mechanism that prevents them, stating where the idempotency key comes from and how it is stored. (2 marks)",
					"The service also publishes a 'payment completed' event after updating its database. Explain the dual-write problem and how the transactional outbox pattern solves it. (2 marks)",
				}},
		}},
}

func main() {
	dir := "docs/exam-paper"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	check()
	must(os.MkdirAll(dir, 0o755))
	write(filepath.Join(dir, "computing-comprehensive-exam.html"), []byte(renderHTML()))
	imp := renderImport()
	verifyImport(imp)
	write(filepath.Join(dir, "computing-comprehensive-exam-import.pdf"), imp)
}

// check enforces the paper specification.
func check() {
	want := map[string][2]int{"A": {12, 40}, "B": {10, 20}, "C": {8, 40}}
	perArea := make([]int, len(areas))
	total, n := 0, 0
	for _, s := range sections {
		marks := 0
		for _, q := range s.Questions {
			marks += q.Marks
			perArea[q.Area]++
			got := strings.Count(q.Prompt, blank)
			if s.Letter == "B" && (got < 2 || got > 3 || got != len(q.Blanks)) {
				fail("Section B question %q has %d blanks and %d answers (need 2-3)", q.Prompt[:40], got, len(q.Blanks))
			}
			if s.Letter != "B" && got > 0 {
				fail("blank outside Section B in %q", q.Prompt[:40])
			}
		}
		w := want[s.Letter]
		if len(s.Questions) != w[0] || marks != w[1] {
			fail("Section %s has %d questions / %d marks; want %d / %d", s.Letter, len(s.Questions), marks, w[0], w[1])
		}
		total += marks
		n += len(s.Questions)
	}
	for i, c := range perArea {
		if c != 3 {
			fail("%s has %d questions; want 3", areas[i], c)
		}
	}
	if n != 30 || total != 100 {
		fail("paper has %d questions and %d marks; want 30 and 100", n, total)
	}
	fmt.Printf("checked: %d questions, %d marks, 3 per area, sections 12/10/8 = 40/20/40\n", n, total)
}

// --- Print layout ----------------------------------------------------------------

// labelledBlankRe matches "(ii) ______" so the label never wraps away from its line.
var labelledBlankRe = regexp.MustCompile(`\((i{1,3})\) ` + regexp.QuoteMeta(blank))

func renderHTML() string {
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Faculty of Computing Comprehensive Examination</title><style>` + css + `</style></head><body>`)
	b.WriteString(`<section class="cover">
<p class="uni">[UNIVERSITY NAME]</p>
<p class="fac">Faculty of Computing</p>
<p class="dept">Department: [DEPARTMENT NAME]</p>
<hr class="rule">
<h1>Faculty of Computing<br>Comprehensive Examination</h1>
<p class="level">500-Level Undergraduate and Master's</p>
<p class="session">Academic Session: [ACADEMIC SESSION]</p>
<table class="facts"><tr><td>Duration</td><td><b>30 minutes</b></td><td>Total marks</td><td><b>100</b></td></tr>
<tr><td>Questions</td><td><b>30</b></td><td>Sections</td><td><b>A, B and C</b></td></tr></table>
<table class="candidate">
<tr><td>Candidate name</td><td class="fill"></td></tr>
<tr><td>Matriculation number</td><td class="fill"></td></tr>
<tr><td>Programme</td><td class="fill"></td></tr>
<tr><td>Examination date</td><td class="fill"></td></tr>
</table>
<h2 class="instr-h">Instructions to candidates</h2>
<ol class="instr">
<li>Answer <b>ALL</b> questions in Sections A, B and C.</li>
<li>Section A (Questions 1&ndash;12): theory and analysis, 40 marks.</li>
<li>Section B (Questions 13&ndash;22): fill in the gaps, 20 marks.</li>
<li>Section C (Questions 23&ndash;30): applied technical problems, 40 marks.</li>
<li>Marks for each question are shown beside it. Write your answers in the spaces provided.</li>
<li>No calculators, computers, phones or other electronic devices are permitted.</li>
<li>Write legibly in blue or black ink. Do not write your name anywhere except on this cover page.</li>
</ol>
<table class="tally"><tr><th>Section</th><th>A</th><th>B</th><th>C</th><th>Total</th></tr>
<tr><td>Maximum</td><td>40</td><td>20</td><td>40</td><td>100</td></tr>
<tr><td>Awarded</td><td></td><td></td><td></td><td></td></tr></table>
<p class="donot">Do not turn over until you are told to begin.</p>
</section>`)
	num := 0
	for _, s := range sections {
		marks := 0
		for _, q := range s.Questions {
			marks += q.Marks
		}
		fmt.Fprintf(&b, `<section class="sec"><h2>Section %s &mdash; %s <span class="sm">[%d marks]</span></h2><p class="sec-i">%s</p>`,
			s.Letter, html.EscapeString(s.Title), marks, html.EscapeString(s.Instructions))
		for _, q := range s.Questions {
			num++
			fmt.Fprintf(&b, `<article class="q"><div class="qh"><span class="qn">%d.</span><div class="qt">`, num)
			prompt := html.EscapeString(q.Prompt)
			if s.Letter == "B" {
				prompt = labelledBlankRe.ReplaceAllString(prompt, `<span class="nw">($1) <span class="gap"></span></span>`)
			}
			fmt.Fprintf(&b, `<p>%s</p>`, prompt)
			if q.Code != "" {
				fmt.Fprintf(&b, `<pre>%s</pre>`, html.EscapeString(q.Code))
			}
			if len(q.Parts) > 0 {
				b.WriteString(`<ol class="parts">`)
				for _, p := range q.Parts {
					fmt.Fprintf(&b, `<li>%s</li>`, html.EscapeString(p))
				}
				b.WriteString(`</ol>`)
			}
			fmt.Fprintf(&b, `</div><span class="qm">[%d marks]</span></div>`, q.Marks)
			if s.Letter == "B" {
				b.WriteString(`<div class="gapans">`)
				for i := range q.Blanks {
					fmt.Fprintf(&b, `<div class="ga"><span>(%s)</span><span class="line"></span></div>`, roman(i+1))
				}
				b.WriteString(`</div>`)
			} else {
				b.WriteString(`<div class="lines">` + strings.Repeat(`<div class="wl"></div>`, q.Lines) + `</div>`)
			}
			b.WriteString(`</article>`)
		}
		b.WriteString(`</section>`)
	}
	b.WriteString(`<p class="end">&mdash; END OF EXAMINATION &mdash;</p></body></html>`)
	return b.String()
}

func roman(i int) string { return []string{"", "i", "ii", "iii", "iv", "v"}[i] }

const css = `
@page { size: A4; margin: 20mm 18mm 22mm 18mm; }
* { box-sizing: border-box; }
html { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
body { margin: 0; font-family: Cambria, Georgia, "Times New Roman", serif; font-size: 11pt; line-height: 1.45; color: #000; background: #fff; }
.cover { text-align: center; page-break-after: always; padding-top: 6mm; }
.uni { font-size: 18pt; font-weight: bold; letter-spacing: .06em; margin: 0; }
.fac { font-size: 14pt; margin: 4pt 0 0; }
.dept { margin: 2pt 0 0; }
.rule { border: 0; border-top: 1.5pt solid #000; margin: 12pt 0 14pt; }
h1 { font-size: 22pt; line-height: 1.2; margin: 0 0 6pt; }
.level { font-size: 12.5pt; font-style: italic; margin: 0; }
.session { margin: 4pt 0 12pt; }
table { border-collapse: collapse; }
.facts { margin: 0 auto 14pt; }
.facts td { border: 0.75pt solid #000; padding: 4pt 10pt; }
.candidate { width: 100%; margin: 0 0 12pt; text-align: left; }
.candidate td { padding: 9pt 0 2pt; }
.candidate td:first-child { width: 40%; font-weight: bold; }
.candidate .fill { border-bottom: 0.75pt solid #000; }
.instr-h { font-size: 12.5pt; text-align: left; margin: 0 0 4pt; }
.instr { text-align: left; margin: 0 0 12pt; padding-left: 18pt; }
.instr li { margin: 2pt 0; }
.tally { margin: 0 auto 10pt; }
.tally th, .tally td { border: 0.75pt solid #000; padding: 3pt 12pt; min-width: 48pt; height: 18pt; }
.donot { font-weight: bold; margin-top: 10pt; }
.sec h2 { font-size: 14pt; margin: 0 0 3pt; padding-bottom: 3pt; border-bottom: 1pt solid #000; }
.sec { page-break-before: always; }
.sm { font-weight: normal; font-size: 11pt; }
.sec-i { font-style: italic; margin: 4pt 0 10pt; }
.q { margin: 0 0 12pt; break-inside: avoid; }
.qh { display: flex; gap: 6pt; align-items: flex-start; }
.qn { font-weight: bold; min-width: 18pt; }
.qt { flex: 1; }
.qt p { margin: 0; text-align: justify; }
.qm { white-space: nowrap; font-weight: bold; font-size: 10pt; padding-left: 8pt; }
pre { font-family: Consolas, "Courier New", monospace; font-size: 9.5pt; border: 0.75pt solid #000; padding: 5pt 8pt; margin: 5pt 0; white-space: pre-wrap; }
.parts { list-style: none; counter-reset: part; margin: 4pt 0 0; padding-left: 22pt; }
.parts li { margin: 1.5pt 0; counter-increment: part; text-indent: -18pt; padding-left: 18pt; }
.parts li::before { content: "(" counter(part, lower-alpha) ") "; display: inline-block; width: 18pt; text-indent: 0; }
.nw { white-space: nowrap; }
.gap { display: inline-block; width: 70pt; border-bottom: 0.75pt solid #000; height: 10pt; }
.gapans { margin: 6pt 0 0 24pt; }
.ga { display: flex; gap: 6pt; align-items: flex-end; margin: 7pt 0; }
.ga span:first-child { min-width: 22pt; }
.ga .line { flex: 0 0 60%; border-bottom: 0.75pt solid #000; height: 14pt; }
.lines { margin: 4pt 0 0 24pt; }
.wl { height: 21pt; border-bottom: 0.5pt solid #555; }
.end { text-align: center; font-weight: bold; margin-top: 18pt; letter-spacing: .08em; }
`

// --- Import copy -----------------------------------------------------------------

// renderImport writes the staff copy in the format Questions > Import from
// PDF reads: theory as written questions, gaps with their accepted answers.
func renderImport() []byte {
	var L []examimport.Line
	add := func(l examimport.Line) { L = append(L, l) }
	add(examimport.Line{Text: "Faculty of Computing Comprehensive Examination", Size: 20, Bold: true, Gap: 80})
	add(examimport.Line{Text: "STAFF COPY - contains fill-in-the-gap answers. Do not give to candidates.", Size: 12, Bold: true, Gap: 8})
	add(examimport.Line{Text: "Import into Exam Shield: open an exam, choose Questions, then Import from PDF. Set the exam's time limit to 30 minutes. Sections A and C are marked by examiners in the Marking tab; Section B is marked automatically.", Size: 10.5, Gap: 8})
	num := 0
	for _, s := range sections {
		add(examimport.Line{Text: fmt.Sprintf("Section %s: %s", s.Letter, s.Title), Size: 15, Bold: true, PageBreak: num > 0, Gap: 20})
		for _, q := range s.Questions {
			num++
			text := q.Prompt
			if q.Code != "" {
				text += " " + strings.Join(strings.Fields(q.Code), " ")
			}
			for i, p := range q.Parts {
				text += fmt.Sprintf(" (%c) %s", 'a'+i, p)
			}
			keep := 0
			if len(q.Blanks) > 0 {
				keep = 1
			}
			add(examimport.Line{Text: fmt.Sprintf("%d. %s [%d marks]", num, text, q.Marks), Size: 10.5, Bold: true, Gap: 9, KeepNext: keep})
			if len(q.Blanks) > 0 {
				alts := make([]string, len(q.Blanks))
				for i, a := range q.Blanks {
					alts[i] = strings.Join(a, " / ")
				}
				add(examimport.Line{Text: "Answer: " + strings.Join(alts, " | "), Size: 10, Indent: 18})
			}
		}
	}
	return examimport.RenderPDF(L, examimport.RenderOptions{Footer: "Faculty of Computing Comprehensive Examination - staff copy"})
}

// verifyImport reads the staff copy back through the real importer.
func verifyImport(pdf []byte) {
	lines, _, err := examimport.ExtractLines(pdf)
	must(err)
	res := examimport.Parse(lines)
	if len(res.Questions) != 30 {
		fail("import copy parsed to %d questions, want 30", len(res.Questions))
	}
	var marks float64
	kinds := map[string]int{}
	num := 0
	for _, s := range sections {
		for _, q := range s.Questions {
			got := res.Questions[num]
			num++
			marks += got.Marks
			kinds[got.Kind]++
			if len(got.Warnings) > 0 {
				fail("question %d imports with warnings: %v", num, got.Warnings)
			}
			if len(q.Blanks) > 0 && fmt.Sprint(got.Blanks) != fmt.Sprint(q.Blanks) {
				fail("question %d blanks: got %v want %v", num, got.Blanks, q.Blanks)
			}
		}
	}
	if marks != 100 || kinds["TEXT"] != 20 || kinds["GAP"] != 10 {
		fail("import copy: %v marks, kinds %v", marks, kinds)
	}
	fmt.Println("verified: import copy parses to 20 written + 10 fill-in-the-gap questions, 100 marks")
}

func write(path string, data []byte) {
	must(os.WriteFile(path, data, 0o644))
	fmt.Printf("wrote %-55s %6.1f KB\n", path, float64(len(data))/1024)
}

func must(err error) {
	if err != nil {
		fail("%v", err)
	}
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "exampaper: "+format+"\n", a...)
	os.Exit(1)
}
