package html

import (
	"bytes"
	"html"
	"strings"

	"github.com/yuin/goldmark/ast"

	"github.com/donaldgifford/docz/v2/pkg/adr"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/impl"
	"github.com/donaldgifford/docz/v2/pkg/investigation"
	"github.com/donaldgifford/docz/v2/pkg/runbook"
)

// facts are the type packages' readings of the document. Each is parsed
// the first time a region it reads is laid out, never because of the
// document's type name, so a custom type carrying IMPL regions gets phases
// (ADR-0002 R7). A failed parse leaves its field nil, is recorded once in
// Page.Problems, and the regions it would have read are written as text.
type facts struct {
	parsed  map[string]bool
	impl    *impl.Doc
	runbook *runbook.Doc
	adr     *adr.Doc
	inv     *investigation.Doc
}

// once runs parse the first time name is asked for.
func (d *doc) once(name string, parse func() error) {
	if d.facts.parsed == nil {
		d.facts.parsed = make(map[string]bool)
	}

	if d.facts.parsed[name] {
		return
	}

	d.facts.parsed[name] = true

	if err := parse(); err != nil {
		d.problem("%s layout: %w", name, err)
	}
}

func (d *doc) implDoc() *impl.Doc {
	d.once("impl", func() error {
		v, err := impl.Parse(d.src)
		if err == nil {
			d.facts.impl = &v
		}

		return err
	})

	return d.facts.impl
}

func (d *doc) runbookDoc() *runbook.Doc {
	d.once("runbook", func() error {
		v, err := runbook.Parse(d.src)
		if err == nil {
			d.facts.runbook = &v
		}

		return err
	})

	return d.facts.runbook
}

func (d *doc) adrDoc() *adr.Doc {
	d.once("adr", func() error {
		v, err := adr.Parse(d.src)
		if err == nil {
			d.facts.adr = &v
		}

		return err
	})

	return d.facts.adr
}

func (d *doc) invDoc() *investigation.Doc {
	d.once("investigation", func() error {
		v, err := investigation.Parse(d.src)
		if err == nil {
			d.facts.inv = &v
		}

		return err
	})

	return d.facts.inv
}

// verdictClass is the verdict's modifier class: yes, no, or partial for an
// inconclusive answer, none for one that opens with none of them.
func verdictClass(v investigation.Verdict) string {
	switch v {
	case investigation.VerdictYes:
		return " yes"
	case investigation.VerdictNo:
		return " no"
	case investigation.VerdictInconclusive:
		return " partial"
	case investigation.VerdictUnknown:
	}

	return ""
}

// filterBar is the task filter the runtime drives, once per Phases panel.
const filterBar = `<div class="filters" role="group" aria-label="Show tasks">` +
	`<button type="button" data-filter="all">All</button>` +
	`<button type="button" data-filter="open">Open</button>` +
	`<button type="button" data-filter="done">Done</button></div>` + "\n"

// phase writes one IMPL phase as a card: its heading and progress, the
// bar, its prose, its tasks with their state, and its criteria in an ok
// callout. Progress counts as impl.Doc.Progress does: checked tasks over
// every task but the skipped.
func (d *doc) phase(w *bytes.Buffer, s section) (bool, error) {
	r, ok := d.region(s)
	doc := d.implDoc()

	if !ok || doc == nil {
		return false, nil
	}

	var ph *impl.Phase

	for i := range doc.Phases {
		if doc.Phases[i].Line >= s.Body && doc.Phases[i].Line <= s.Through {
			ph = &doc.Phases[i]

			break
		}
	}

	h := d.headingAt(lineOr(ph))
	if ph == nil || h == nil {
		return false, nil
	}

	if !d.filtered {
		put(w, filterBar)

		d.filtered = true
	}

	done, total := phaseProgress(ph)

	title, err := d.headingText(h)
	if err != nil {
		return false, err
	}

	put(w, `<div class="card phase"`+idAttr(h)+">\n")
	put(w, `<div class="phase-head"><h3>`)
	w.Write(title)
	put(w, "</h3>")
	progressBar(w, done, total)

	tasks := d.childrenOf(r, "tasks")
	criteria := d.childrenOf(r, "criteria")

	replace := make(map[int]replacer)

	for _, tr := range tasks {
		d.skipHeading(tr)
		d.replaceLists(replace, tr, func(w *bytes.Buffer, _ ast.Node, id string) error {
			return d.phaseTasks(w, ph, tr, id)
		})
	}

	from := h.Line + 1

	for _, cr := range criteria {
		if err := d.textWith(w, from, cr.Start-1, replace); err != nil {
			return false, err
		}

		ch := d.headingIn(cr.Start+1, cr.End-1)
		d.skipHeading(cr)

		put(w, `<div class="callout ok"`+idAttr(ch)+`><p class="callout-title">Success criteria</p>`+"\n")

		if err := d.text(w, cr.Start+1, cr.End-1); err != nil {
			return false, err
		}

		put(w, "</div>\n")

		from = cr.End + 1
	}

	if err := d.textWith(w, from, s.Through, replace); err != nil {
		return false, err
	}

	put(w, "</div>\n")

	return true, nil
}

// lineOr is a phase's heading line, 0 for none.
func lineOr(ph *impl.Phase) int {
	if ph == nil {
		return 0
	}

	return ph.Line
}

// skipHeading leaves a child region's heading out of the text: the element
// that stands for the region carries its anchor instead.
func (d *doc) skipHeading(r docparse.Region) {
	if h := d.headingIn(r.Start+1, r.End-1); h != nil {
		d.skip[h.Line] = true
	}
}

// replaceLists registers write for each top-level list inside r, passing
// the region heading's anchor as an id attribute for the first.
func (d *doc) replaceLists(replace map[int]replacer, r docparse.Region, write func(*bytes.Buffer, ast.Node, string) error) {
	id := idAttr(d.headingIn(r.Start+1, r.End-1))

	for n := d.md.root.FirstChild(); n != nil; n = n.NextSibling() {
		if _, ok := n.(*ast.List); !ok {
			continue
		}

		line := d.md.startLine(n)
		if line <= r.Start || line >= r.End {
			continue
		}

		listID := id
		id = ""
		replace[line] = func(w *bytes.Buffer, n ast.Node) error { return write(w, n, listID) }
	}
}

// phaseTasks writes the tasks of one tasks region in a phase.
func (d *doc) phaseTasks(w *bytes.Buffer, ph *impl.Phase, r docparse.Region, id string) error {
	put(w, `<ul class="tasks"`+id+">\n")

	for _, t := range ph.Tasks {
		if t.Line <= r.Start || t.Line >= r.End {
			continue
		}

		state, checked := "open", ""

		switch {
		case t.Skipped != nil:
			state = "skipped"
		case t.Deferred != nil:
			state = "deferred"
		case t.Checked:
			state, checked = "done", " checked"
		}

		text, err := d.md.inline(t.Text, t.Line)
		if err != nil {
			return err
		}

		put(w, `<li data-state="`+state+`"><input type="checkbox"`+checked+` disabled><span class="text">`)

		if t.Skipped != nil {
			put(w, "<del>")
			w.Write(text)
			put(w, "</del>")
		} else {
			w.Write(text)
		}

		if m := marker(&t); m != nil {
			put(w, ` <span class="pill warn">`+state+`</span>`)

			if m.Note != "" {
				note, err := d.md.inline(m.Note, m.Line)
				if err != nil {
					return err
				}

				put(w, ` <span class="note">`)
				w.Write(note)
				put(w, "</span>")
			}
		}

		put(w, "</span></li>\n")
	}

	put(w, "</ul>\n")

	return nil
}

// phaseProgress counts a phase as impl.Doc.Progress does: checked tasks
// over every task but the skipped.
func phaseProgress(ph *impl.Phase) (done, total int) {
	for i := range ph.Tasks {
		if ph.Tasks[i].Skipped != nil {
			continue
		}

		total++

		if ph.Tasks[i].Checked {
			done++
		}
	}

	return done, total
}

// marker is a task's skipped or deferred marker, nil for neither.
func marker(t *impl.Task) *impl.Marker {
	if t.Skipped != nil {
		return t.Skipped
	}

	return t.Deferred
}

// procedure writes one runbook procedure: its heading and prose as text,
// then its steps numbered, its verification as open tasks, and its
// rollback numbered in a bad callout, each under its own heading.
func (d *doc) procedure(w *bytes.Buffer, s section) (bool, error) {
	r, ok := d.region(s)
	doc := d.runbookDoc()

	if !ok || doc == nil {
		return false, nil
	}

	var pr *runbook.Procedure

	for i := range doc.Procedures {
		if doc.Procedures[i].Line > r.Start && doc.Procedures[i].Line < r.End {
			pr = &doc.Procedures[i]

			break
		}
	}

	if pr == nil {
		return false, nil
	}

	replace := make(map[int]replacer)

	for _, c := range d.childrenOf(r, "steps") {
		d.replaceLists(replace, c, func(w *bytes.Buffer, _ ast.Node, _ string) error {
			return d.steps(w, pr.Steps, c)
		})
	}

	for _, c := range d.childrenOf(r, "verification") {
		d.replaceLists(replace, c, func(w *bytes.Buffer, n ast.Node, _ string) error {
			return d.openTasks(w, n, "")
		})
	}

	for _, c := range d.childrenOf(r, "rollback") {
		d.replaceLists(replace, c, func(w *bytes.Buffer, _ ast.Node, _ string) error {
			put(w, `<div class="callout bad"><p class="callout-title">Rollback</p>`+"\n")

			if err := d.steps(w, pr.Rollback, c); err != nil {
				return err
			}

			put(w, "</div>\n")

			return nil
		})
	}

	return true, d.textWith(w, s.Body, s.Through, replace)
}

// steps writes the steps of one region as ol.steps, children nested.
func (d *doc) steps(w *bytes.Buffer, steps []runbook.Step, r docparse.Region) error {
	put(w, `<ol class="steps">`+"\n")

	for i := range steps {
		st := &steps[i]
		if st.Line <= r.Start || st.Line >= r.End {
			continue
		}

		if err := d.step(w, st); err != nil {
			return err
		}
	}

	put(w, "</ol>\n")

	return nil
}

// step writes one step: its id, text, commands, expected outcome, and
// children.
func (d *doc) step(w *bytes.Buffer, st *runbook.Step) error {
	id := "step-" + strings.ToLower(strings.ReplaceAll(st.ID, ".", "-"))

	text, err := d.md.inline(st.Text, st.Line)
	if err != nil {
		return err
	}

	put(w, `<li id="`+html.EscapeString(id)+`"><span class="step-id">`+html.EscapeString(st.ID)+"</span> ")
	w.Write(text)
	put(w, "\n")

	for _, c := range st.Commands {
		writeCode(w, c.Lang, []byte(c.Body+"\n"))
	}

	if st.Expected != "" {
		exp, err := d.md.inline(st.Expected, st.Line)
		if err != nil {
			return err
		}

		put(w, `<p class="expected"><b>Expected:</b> `)
		w.Write(exp)
		put(w, "</p>\n")
	}

	if len(st.Children) > 0 {
		put(w, `<ol class="steps">`+"\n")

		for i := range st.Children {
			if err := d.step(w, &st.Children[i]); err != nil {
				return err
			}
		}

		put(w, "</ol>\n")
	}

	put(w, "</li>\n")

	return nil
}

// scenario writes one runbook scenario as a closed expandable: the
// symptom as its summary, the symptom, alert, and likely cause as a
// definition list, then its steps.
func (d *doc) scenario(w *bytes.Buffer, s section) (bool, error) {
	r, ok := d.region(s)
	doc := d.runbookDoc()

	if !ok || doc == nil {
		return false, nil
	}

	var sc *runbook.Scenario

	for i := range doc.Scenarios {
		if doc.Scenarios[i].Line > r.Start && doc.Scenarios[i].Line < r.End {
			sc = &doc.Scenarios[i]

			break
		}
	}

	h := d.headingAt(scenarioLine(sc))
	if sc == nil || h == nil {
		return false, nil
	}

	put(w, `<details class="expand"`+idAttr(h)+"><summary>"+html.EscapeString(scenarioName(sc, h))+"</summary>\n")
	put(w, `<div class="body">`+"\n<dl>\n")

	for _, f := range [][2]string{{"Symptom", sc.Symptom}, {"Alert", sc.Alert}, {"Likely cause", sc.LikelyCause}} {
		if strings.TrimSpace(f[1]) == "" {
			continue
		}

		v, err := d.md.inline(f[1], sc.Line)
		if err != nil {
			return false, err
		}

		put(w, "<dt>"+f[0]+"</dt><dd>")
		w.Write(v)
		put(w, "</dd>\n")
	}

	put(w, "</dl>\n")

	// The fields are in the definition list; the rest of the scenario's
	// prose, and its steps, follow.
	for n := h.Line + 1; n < r.End; n++ {
		if fieldLine.MatchString(d.lines[n-1]) {
			d.skip[n] = true
		}
	}

	d.skip[h.Line] = true

	replace := make(map[int]replacer)

	for _, c := range d.childrenOf(r, "steps") {
		d.replaceLists(replace, c, func(w *bytes.Buffer, _ ast.Node, _ string) error {
			return d.steps(w, sc.Steps, c)
		})
	}

	if err := d.textWith(w, s.Body, s.Through, replace); err != nil {
		return false, err
	}

	put(w, "</div>\n</details>\n")

	return true, nil
}

// scenarioLine is a scenario's heading line, 0 for none.
func scenarioLine(sc *runbook.Scenario) int {
	if sc == nil {
		return 0
	}

	return sc.Line
}

// scenarioName is the scenario's name without its "Scenario:" prefix.
func scenarioName(sc *runbook.Scenario, h *docparse.Heading) string {
	if sc.Symptom != "" {
		return sc.Symptom
	}

	t := strings.TrimSpace(stripComments(h.Text))
	if i := strings.Index(t, ":"); i >= 0 {
		return strings.TrimSpace(t[i+1:])
	}

	return t
}
