package html

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/util"
	"go.yaml.in/yaml/v3"
)

// A widget is a fenced block whose info string names it and whose body is
// YAML (DESIGN-0025 §6, the docz-html skill's widgets.md). Each schema is
// closed: an unknown key, a missing required key, or a value of the wrong
// type makes the block invalid, and an invalid block is written as its
// source under a caution callout rather than drawn.

// widgetNames maps a fence's info string to the widget's short name.
var widgetNames = map[string]string{
	"docz-flow":      "flow",
	"docz-stepper":   "stepper",
	"docz-scorecard": "scorecard",
}

// widget is a decoded, validated block.
type widget interface {
	// caption is the figure's caption, "" for none.
	caption() string
	// fallback writes the widget readable without JavaScript.
	fallback(w io.Writer, tabs map[string]string)
	// check reports what the field checks find wrong.
	check() []string
}

// flow is a docz-flow block.
type flow struct {
	Title  string      `json:"title,omitempty"  yaml:"title"`
	Stages []flowStage `json:"stages"           yaml:"stages"`
	Gates  []int       `json:"gates,omitempty"  yaml:"gates"`
	Label  string      `json:"label,omitempty"  yaml:"label"`
	Legend string      `json:"legend,omitempty" yaml:"legend"`
	Alert  string      `json:"alert,omitempty"  yaml:"alert"`
}

type flowStage struct {
	Name string `json:"name"           yaml:"name"`
	Does string `json:"does"           yaml:"does"`
	Note string `json:"note,omitempty" yaml:"note"`
}

// stepper is a docz-stepper block.
type stepper struct {
	Title     string            `json:"title,omitempty" yaml:"title"`
	Steps     []stepperStep     `json:"steps"           yaml:"steps"`
	Scenarios []stepperScenario `json:"scenarios"       yaml:"scenarios"`
}

type stepperStep struct {
	Key   string `json:"key"             yaml:"key"`
	Name  string `json:"name"            yaml:"name"`
	Title string `json:"title,omitempty" yaml:"title"`
	Does  string `json:"does"            yaml:"does"`
}

type stepperScenario struct {
	Name      string            `json:"name"                 yaml:"name"`
	FailsAt   string            `json:"fails_at,omitempty"   yaml:"fails_at"`
	FailLabel string            `json:"fail_label,omitempty" yaml:"fail_label"`
	Notes     map[string]string `json:"notes,omitempty"      yaml:"notes"`
}

// scorecard is a docz-scorecard block.
type scorecard struct {
	Title   string        `json:"title,omitempty"   yaml:"title"`
	Note    string        `json:"note,omitempty"    yaml:"note"`
	Checks  []scoreCheck  `json:"checks"            yaml:"checks"`
	Example *scoreResults `json:"example,omitempty" yaml:"example"`
}

type scoreCheck struct {
	ID     string   `json:"id"               yaml:"id"`
	Name   string   `json:"name"             yaml:"name"`
	Target string   `json:"target"           yaml:"target"`
	Weight *float64 `json:"weight,omitempty" yaml:"weight"`
	Fixes  string   `json:"fixes,omitempty"  yaml:"fixes"`
	Source string   `json:"source,omitempty" yaml:"source"`
}

type scoreResults struct {
	GeneratedAt string        `json:"generated_at,omitempty" yaml:"generated_at"`
	Subject     string        `json:"subject,omitempty"      yaml:"subject"`
	Results     []scoreResult `json:"results"                yaml:"results"`
}

// scoreResult is one measured check. Score is required but may be null,
// which a plain decode cannot tell from absent, so the result reads its
// own mapping.
type scoreResult struct {
	ID       string   `json:"id"                 yaml:"id"`
	Score    *float64 `json:"score"              yaml:"score"`
	Observed string   `json:"observed,omitempty" yaml:"observed"`
	Evidence string   `json:"evidence,omitempty" yaml:"evidence"`

	hasScore bool
}

// resultKeys are a result's keys: a custom UnmarshalYAML does not inherit
// the decoder's KnownFields, so it checks them itself.
var resultKeys = map[string]bool{"id": true, "score": true, "observed": true, "evidence": true}

// UnmarshalYAML implements yaml.Unmarshaler.
func (r *scoreResult) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return &yaml.TypeError{Errors: []string{fmt.Sprintf("line %d: a result is a mapping", n.Line)}}
	}

	var bad []string

	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		if !resultKeys[k.Value] {
			bad = append(bad, fmt.Sprintf("line %d: field %s not found", k.Line, k.Value))
		}

		if k.Value == "score" {
			r.hasScore = true
		}
	}

	if bad != nil {
		return &yaml.TypeError{Errors: bad}
	}

	type plain scoreResult

	return n.Decode((*plain)(r))
}

// decodeWidget decodes and checks a block's YAML, returning the widget
// and its JSON island, or every error found.
func decodeWidget(name string, body []byte) (widget, []byte, []string) {
	var v widget

	switch name {
	case "flow":
		v = &flow{}
	case "stepper":
		v = &stepper{}
	default:
		v = &scorecard{}
	}

	dec := yaml.NewDecoder(bytes.NewReader(body))
	dec.KnownFields(true)

	if err := dec.Decode(v); err != nil {
		var te *yaml.TypeError
		if errors.As(err, &te) {
			return nil, nil, te.Errors
		}

		if errors.Is(err, io.EOF) {
			return nil, nil, []string{"the block is empty"}
		}

		return nil, nil, []string{strings.TrimPrefix(err.Error(), "yaml: ")}
	}

	if errs := v.check(); errs != nil {
		return nil, nil, errs
	}

	island, err := json.Marshal(v)
	if err != nil {
		return nil, nil, []string{err.Error()}
	}

	return v, island, nil
}

func (f *flow) caption() string { return f.Title }

func (f *flow) check() []string {
	var errs []string

	if n := len(f.Stages); n < 2 || n > 8 {
		errs = append(errs, fmt.Sprintf("stages: needs 2 to 8 stages, has %d", n))
	}

	for i, s := range f.Stages {
		errs = required(errs, fmt.Sprintf("stages[%d]", i), "name", s.Name, "does", s.Does)
	}

	for _, g := range f.Gates {
		if g < 1 || g > len(f.Stages) {
			errs = append(errs, fmt.Sprintf("gates: %d is not a stage between 1 and %d", g, len(f.Stages)))
		}
	}

	return errs
}

func (s *stepper) caption() string { return s.Title }

// stepKey is a step's key's shape.
var stepKey = regexp.MustCompile(`^[a-z0-9-]+$`)

func (s *stepper) check() []string {
	var errs []string

	if n := len(s.Steps); n < 2 || n > 12 {
		errs = append(errs, fmt.Sprintf("steps: needs 2 to 12 steps, has %d", n))
	}

	keys := make(map[string]bool, len(s.Steps))

	for i, st := range s.Steps {
		at := fmt.Sprintf("steps[%d]", i)
		errs = required(errs, at, "key", st.Key, "name", st.Name, "does", st.Does)

		switch {
		case st.Key == "":
		case !stepKey.MatchString(st.Key):
			errs = append(errs, fmt.Sprintf("%s.key: %q is not [a-z0-9-]+", at, st.Key))
		case keys[st.Key]:
			errs = append(errs, fmt.Sprintf("%s.key: %q is used twice", at, st.Key))
		}

		keys[st.Key] = true
	}

	if len(s.Scenarios) == 0 {
		errs = append(errs, "scenarios: needs at least one scenario")
	}

	names := make(map[string]bool, len(s.Scenarios))

	for i, sc := range s.Scenarios {
		at := fmt.Sprintf("scenarios[%d]", i)
		errs = required(errs, at, "name", sc.Name)

		if sc.Name != "" && names[sc.Name] {
			errs = append(errs, fmt.Sprintf("%s.name: %q is used twice", at, sc.Name))
		}

		names[sc.Name] = true

		if sc.FailsAt != "" && !keys[sc.FailsAt] {
			errs = append(errs, fmt.Sprintf("%s.fails_at: %q is no step's key", at, sc.FailsAt))
		}

		for _, k := range slices.Sorted(maps.Keys(sc.Notes)) {
			if !keys[k] {
				errs = append(errs, fmt.Sprintf("%s.notes: %q is no step's key", at, k))
			}
		}
	}

	return errs
}

func (c *scorecard) caption() string { return c.Title }

func (c *scorecard) check() []string {
	var errs []string

	if len(c.Checks) == 0 {
		errs = append(errs, "checks: needs at least one check")
	}

	ids := make(map[string]bool, len(c.Checks))

	for i, ck := range c.Checks {
		at := fmt.Sprintf("checks[%d]", i)
		errs = required(errs, at, "id", ck.ID, "name", ck.Name, "target", ck.Target)

		if ck.ID != "" && ids[ck.ID] {
			errs = append(errs, fmt.Sprintf("%s.id: %q is used twice", at, ck.ID))
		}

		ids[ck.ID] = true

		if ck.Weight != nil && *ck.Weight <= 0 {
			errs = append(errs, fmt.Sprintf("%s.weight: must be greater than 0", at))
		}
	}

	if c.Example != nil {
		errs = append(errs, c.Example.check(ids)...)
	}

	return errs
}

// check reports what is wrong with the example results, given the
// checks' ids.
func (r *scoreResults) check(ids map[string]bool) []string {
	var errs []string

	if r.Results == nil {
		errs = append(errs, "example.results: required")
	}

	for i, res := range r.Results {
		at := fmt.Sprintf("example.results[%d]", i)
		errs = required(errs, at, "id", res.ID)

		if res.ID != "" && !ids[res.ID] {
			errs = append(errs, fmt.Sprintf("%s.id: %q is no check's id", at, res.ID))
		}

		switch {
		case !res.hasScore:
			errs = append(errs, at+".score: required")
		case res.Score != nil && (*res.Score < 0 || *res.Score > 1):
			errs = append(errs, at+".score: must be between 0 and 1, or null")
		}
	}

	return errs
}

// required appends an error for each empty value in pairs of key, value.
func required(errs []string, at string, pairs ...string) []string {
	for i := 0; i+1 < len(pairs); i += 2 {
		if strings.TrimSpace(pairs[i+1]) == "" {
			errs = append(errs, at+"."+pairs[i]+": required")
		}
	}

	return errs
}

// esc is text escaped for HTML.
func esc(s string) string { return html.EscapeString(s) }

func (f *flow) fallback(w io.Writer, _ map[string]string) {
	put(w, `<div class="widget-fallback">`+"\n")

	if f.Label != "" {
		put(w, `<p class="note">`+esc(f.Label)+"</p>\n")
	}

	gates := make(map[int]bool, len(f.Gates))
	for _, g := range f.Gates {
		gates[g] = true
	}

	put(w, "<ol>\n")

	for i, s := range f.Stages {
		put(w, "<li><b>"+esc(s.Name)+"</b>")

		if gates[i+1] {
			put(w, ` <span class="pill ok">gate</span>`)
		}

		put(w, ": "+esc(s.Does))

		if s.Note != "" {
			put(w, ` <span class="note">`+esc(s.Note)+"</span>")
		}

		put(w, "</li>\n")
	}

	put(w, "</ol>\n")

	if f.Legend != "" {
		put(w, `<p class="note">`+esc(f.Legend)+"</p>\n")
	}

	if f.Alert != "" {
		put(w, `<div class="callout bad"><p>`+esc(f.Alert)+"</p></div>\n")
	}

	put(w, "</div>\n")
}

func (s *stepper) fallback(w io.Writer, _ map[string]string) {
	put(w, `<div class="widget-fallback table-wrap">`+"\n<table>\n<thead><tr><th>Step</th><th>What it does</th>")

	for _, sc := range s.Scenarios {
		put(w, "<th>"+esc(sc.Name)+"</th>")
	}

	put(w, "</tr></thead>\n<tbody>\n")

	failed := make([]int, len(s.Scenarios))

	for j, sc := range s.Scenarios {
		failed[j] = -1

		for i, st := range s.Steps {
			if st.Key == sc.FailsAt {
				failed[j] = i
			}
		}
	}

	for i, st := range s.Steps {
		put(w, "<tr><td>"+esc(st.Name)+"</td><td>"+esc(st.Does)+"</td>")

		for j, sc := range s.Scenarios {
			note := sc.Notes[st.Key]

			switch {
			case failed[j] >= 0 && i == failed[j]:
				label := sc.FailLabel
				if label == "" {
					label = "Denied"
				}

				if note != "" {
					label += ": " + note
				}

				put(w, `<td class="high">`+esc(label)+"</td>")
			case failed[j] >= 0 && i > failed[j]:
				put(w, "<td>Never reached</td>")
			default:
				put(w, "<td>"+esc(note)+"</td>")
			}
		}

		put(w, "</tr>\n")
	}

	put(w, "</tbody>\n</table>\n</div>\n")
}

func (c *scorecard) fallback(w io.Writer, tabs map[string]string) {
	put(w, `<div class="widget-fallback table-wrap">`+"\n<table>\n<thead><tr><th>#</th><th>Check</th>"+
		"<th>Target</th><th>Weight</th><th>Fixes it</th></tr></thead>\n<tbody>\n")

	for _, ck := range c.Checks {
		weight := 1.0
		if ck.Weight != nil {
			weight = *ck.Weight
		}

		fixes := esc(ck.Fixes)
		if key, ok := tabs[strings.ToLower(strings.TrimSpace(ck.Fixes))]; ok {
			fixes = `<a href="#p-` + esc(key) + `" data-tab="` + esc(key) + `">` + fixes + "</a>"
		}

		put(w, "<tr><td>"+esc(ck.ID)+"</td><td>"+esc(ck.Name)+"</td><td>"+esc(ck.Target)+"</td><td>"+
			strconv.FormatFloat(weight, 'f', -1, 64)+"</td><td>"+fixes+"</td></tr>\n")
	}

	put(w, "</tbody>\n</table>\n</div>\n")
}

// yamlLine finds yaml.v3's "line N" so it can be moved to the document's
// line.
var yamlLine = regexp.MustCompile(`\bline (\d+)\b`)

// typeName drops yaml.v3's Go type names from an error.
var typeName = regexp.MustCompile(` (in|into) (type )?(\[\])?\*?html\.\w+`)

// documentErrors rewrites decodeWidget's errors as a reader sees them:
// yaml.v3's Go type names dropped, each "line N" of the block moved to
// the document's line, and an error with no line given the fence's own.
// first is the document line of the block's first YAML line. docz-site
// ports this and decodeWidget to TypeScript; testdata/widgets holds the
// two to the same output.
func documentErrors(errs []string, first int) []string {
	out := make([]string, len(errs))

	for i, e := range errs {
		e = typeName.ReplaceAllString(e, "")
		e = yamlLine.ReplaceAllStringFunc(e, func(m string) string {
			n, _ := strconv.Atoi(yamlLine.FindStringSubmatch(m)[1]) //nolint:errcheck // \d+ always parses.

			return "line " + strconv.Itoa(first+n-1)
		})

		if !yamlLine.MatchString(e) {
			e = "line " + strconv.Itoa(first-1) + ": " + e
		}

		out[i] = e
	}

	return out
}

// writeWidget writes a widget fence: the figure with its island, mount,
// and fallback, or, for an invalid block, a caution callout naming each
// error and the source as code, recorded in problems with its line. first
// is the document line of the block's first YAML line.
func (nr *nodeRenderer) writeWidget(w util.BufWriter, info string, body []byte, first int) {
	name := widgetNames[info]

	v, island, errs := decodeWidget(name, body)
	if errs != nil {
		errs = documentErrors(errs, first)
		for _, e := range errs {
			nr.problems = append(nr.problems, fmt.Errorf("html: invalid %s block: %s", info, e))
		}

		put(w, `<div class="callout caution"><p class="callout-title">Invalid `+info+" block</p>\n<ul>\n")

		for _, e := range errs {
			put(w, "<li>"+esc(e)+"</li>\n")
		}

		put(w, "</ul>\n</div>\n")
		writeCode(w, "yaml", body)

		return
	}

	put(w, `<figure class="widget" data-widget="`+name+`">`+"\n")

	if c := v.caption(); c != "" {
		put(w, "<figcaption>"+esc(c)+"</figcaption>\n")
	}

	// json.Marshal writes <, >, and & as \u003c, \u003e, and \u0026, so
	// no string in the island can close its script element.
	put(w, `<script type="application/json" data-widget="`+name+`">`+string(island)+"</script>\n")
	put(w, `<div class="widget-mount"></div>`+"\n")
	v.fallback(w, nr.tabs)
	put(w, "</figure>\n")
}
