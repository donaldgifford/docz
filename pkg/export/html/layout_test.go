package html

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/doctemplate"
)

// placedByParent are kinds that only ever sit inside another region and
// are laid out by their parent's component, so they need no row: a
// consequence list by consequences, the scope lists by scope, a phase's
// tasks by phase, and a procedure's or scenario's steps, verification, and
// rollback by theirs. The ToC is dropped, and the README index is a
// README's span, never a document's.
var placedByParent = map[string]bool{
	"positive": true, "negative": true, "neutral": true,
	"in-scope": true, "out-of-scope": true,
	"tasks": true, "steps": true, "verification": true, "rollback": true,
	"toc": true, "index": true,
}

// catalogueKinds reads the kinds validate's catalogue lists from its
// source, since the map is unexported: each quoted key, and each constant
// key resolved through the package's own kind constants.
func catalogueKinds(t *testing.T) []string {
	t.Helper()

	dir := filepath.Join("..", "..", "doczcore", "validate")

	src, err := os.ReadFile(filepath.Join(dir, "kindrule.go"))
	if err != nil {
		t.Fatal(err)
	}

	body := string(src)
	start := strings.Index(body, "var catalogue = map[string]KindRule{")
	end := strings.Index(body[start:], "\n}\n")
	if start < 0 || end < 0 {
		t.Fatal("kindrule.go has no catalogue")
	}

	block := body[start : start+end]

	consts := map[string]string{"docparse.TocKind": docparse.TocKind, "docparse.IndexKind": docparse.IndexKind}
	for _, m := range regexp.MustCompile(`(?m)^\s*(kind\w+)\s*=\s*"([a-z-]+)"`).FindAllStringSubmatch(body, -1) {
		consts[m[1]] = m[2]
	}

	var out []string

	for _, m := range regexp.MustCompile(`(?m)^\s*("([a-z-]+)"|([\w.]+))\s*:\s*\{`).FindAllStringSubmatch(block, -1) {
		switch {
		case m[2] != "":
			out = append(out, m[2])
		case consts[m[3]] != "":
			out = append(out, consts[m[3]])
		default:
			t.Fatalf("catalogue key %s resolves to no kind", m[3])
		}
	}

	if len(out) < 40 {
		t.Fatalf("read %d catalogue kinds; the reader has lost the map", len(out))
	}

	return out
}

// renderedTemplate is a built-in's embedded template, rendered.
func renderedTemplate(t *testing.T, typ string) []byte {
	t.Helper()

	tmpl, err := doctemplate.Resolve(typ, "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	out, err := doctemplate.Render(tmpl, &doctemplate.Data{
		Number: "0001", Title: "Title", Date: "2026-10-31", Author: "A",
		Status: "Draft", Type: config.DocType(strings.ToUpper(typ)),
	})
	if err != nil {
		t.Fatal(err)
	}

	return []byte(out)
}

func TestLayoutCoversEveryKind(t *testing.T) {
	t.Parallel()

	kinds := catalogueKinds(t)

	for typ := range specs {
		for _, r := range docparse.Regions(renderedTemplate(t, typ)) {
			kinds = append(kinds, r.Kind)
		}
	}

	for _, k := range kinds {
		if _, ok := layout[k]; !ok && !placedByParent[k] {
			t.Errorf("kind %q has no layout row and is not placed by a parent", k)
		}
	}

	for k := range placedByParent {
		if _, ok := layout[k]; ok {
			t.Errorf("kind %q has a row and is also listed as placed by a parent", k)
		}
	}
}

func TestLayoutTabsPerType(t *testing.T) {
	t.Parallel()

	// DESIGN-0025 §4: the tabs each built-in's template gives, in order.
	want := map[string][]string{
		"rfc":           {"Overview", "Problem", "Proposal", "Alternatives", "Risks", "Success", "References"},
		"adr":           {"Overview", "Decision", "Consequences", "Alternatives", "References"},
		"design":        {"Overview", "Background", "Design", "Interfaces", "Data model", "Testing", "Rollout", "Open questions", "References"},
		"impl":          {"Overview", "Phases", "Changes", "Testing", "Dependencies", "References"},
		"investigation": {"Overview", "Approach", "Findings", "Conclusion", "References"},
		"runbook":       {"Overview", "Procedures", "Scenarios", "Escalation", "References"},
	}

	for typ, tabs := range want {
		t.Run(typ, func(t *testing.T) {
			t.Parallel()

			spec := specs[typ]()
			d := prepare(blankFrontmatter(renderedTemplate(t, typ)), &RenderOptions{Spec: spec})

			got := make([]string, 0, len(d.tabs))
			for _, tb := range d.tabs {
				got = append(got, tb.Label)
			}

			if !slices.Equal(got, tabs) {
				t.Errorf("tabs = %q, want %q", got, tabs)
			}
		})
	}
}
