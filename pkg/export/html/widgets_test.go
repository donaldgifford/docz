package html

import (
	stdhtml "html"
	"regexp"
	"strings"
	"testing"
)

// fence wraps a widget's YAML in its fence after a short document.
func fence(info, yaml string) string {
	return "## Overview\n\nText.\n\n```" + info + "\n" + yaml + "```\n"
}

// islandOf is the page's one widget island, unescaped from nothing: the
// island is written raw.
var islandOf = regexp.MustCompile(`<script type="application/json" data-widget="(\w+)">(.*)</script>`)

func TestRenderWidgets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		src    string
		widget string
		want   []string
	}{
		{
			name: "flow",
			src: fence(
				"docz-flow",
				"title: Request path\ngates: [2]\nlabel: Above\nstages:\n  - name: Ingress\n    does: Terminates TLS\n  - name: Auth\n    does: Verifies the token\n    note: 30 s skew\nlegend: Below\nalert: Breaks\n",
			),
			widget: "flow",
			want: []string{
				`<figcaption>Request path</figcaption>`,
				`"gates":[2]`,
				`<div class="widget-mount"></div>`,
				`<li><b>Auth</b> <span class="pill ok">gate</span>: Verifies the token <span class="note">30 s skew</span></li>`,
				`<p class="note">Above</p>`, `<p class="note">Below</p>`,
				`<div class="callout bad"><p>Breaks</p></div>`,
			},
		},
		{
			name: "stepper",
			src: fence(
				"docz-stepper",
				"steps:\n  - {key: parse, name: Parse, does: Reads it}\n  - {key: verify, name: Verify, does: Checks it}\n"+
					"  - {key: scope, name: Scope, does: Resolves it}\nscenarios:\n  - name: Valid\n    notes: {scope: Two match}\n"+
					"  - name: Expired\n    fails_at: verify\n    fail_label: Rejected\n    notes: {verify: Too old}\n  - name: Bad\n    fails_at: parse\n",
			),
			widget: "stepper",
			want: []string{
				`<th>Valid</th><th>Expired</th><th>Bad</th>`,
				`<tr><td>Parse</td><td>Reads it</td><td></td><td></td><td class="high">Denied</td></tr>`,
				`<tr><td>Verify</td><td>Checks it</td><td></td><td class="high">Rejected: Too old</td><td>Never reached</td></tr>`,
				`<tr><td>Scope</td><td>Resolves it</td><td>Two match</td><td>Never reached</td><td>Never reached</td></tr>`,
			},
		},
		{
			name: "scorecard",
			src: fence(
				"docz-scorecard",
				"checks:\n  - {id: C1, name: Signed, target: All signed, weight: 2, fixes: Overview}\n"+
					"  - {id: C2, name: SBOM, target: One each, fixes: Rollout}\nexample:\n  results:\n    - {id: C1, score: 1}\n    - {id: C2, score: null}\n",
			),
			widget: "scorecard",
			want: []string{
				`<tr><td>C1</td><td>Signed</td><td>All signed</td><td>2</td><td><a href="#p-overview" data-tab="overview">Overview</a></td></tr>`,
				`<tr><td>C2</td><td>SBOM</td><td>One each</td><td>1</td><td>Rollout</td></tr>`,
				`{"id":"C2","score":null}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := page(t, tt.src, &RenderOptions{})
			body := string(p.Body)

			if len(p.Problems) != 0 {
				t.Fatalf("Problems = %v", p.Problems)
			}

			m := islandOf.FindStringSubmatch(body)
			if len(m) < 3 || m[1] != tt.widget {
				t.Fatalf("no %s island in the page", tt.widget)
			}

			if !strings.Contains(body, `<figure class="widget" data-widget="`+tt.widget+`">`) {
				t.Errorf("no figure for the %s widget", tt.widget)
			}

			for _, want := range tt.want {
				if !strings.Contains(body, want) {
					t.Errorf("the page lacks %s", want)
				}
			}
		})
	}
}

func TestRenderWidgetIslandCannotCloseItsScript(t *testing.T) {
	t.Parallel()

	p := page(
		t,
		fence("docz-flow", "stages:\n  - {name: \"</script><b>x\", does: \"a & b\"}\n  - {name: two, does: d}\n"),
		&RenderOptions{},
	)
	island := islandOf.FindStringSubmatch(string(p.Body))

	if island == nil {
		t.Fatalf("no island")
	}

	if strings.Contains(island[2], "<") || !strings.Contains(island[2], `\u003c/script\u003e`) {
		t.Errorf("the island writes < raw: %s", island[2])
	}
}

func TestRenderInvalidWidgets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, src, want string
	}{
		{
			"unknown key",
			fence("docz-flow", "stages:\n  - {name: a, does: b}\n  - {name: c, does: d, colour: red}\n"),
			"line 13: field colour not found",
		},
		{
			"missing required",
			fence("docz-flow", "stages:\n  - {name: a, does: b}\n  - {name: c}\n"),
			"line 10: stages[1].does: required",
		},
		{
			"wrong type",
			fence("docz-flow", "gates: two\nstages:\n  - {name: a, does: b}\n  - {name: c, does: d}\n"),
			"line 11: cannot unmarshal !!str `two`",
		},
		{
			"gate out of range",
			fence("docz-flow", "gates: [3]\nstages:\n  - {name: a, does: b}\n  - {name: c, does: d}\n"),
			"gates: 3 is not a stage between 1 and 2",
		},
		{
			"too few stages",
			fence("docz-flow", "stages:\n  - {name: a, does: b}\n"),
			"stages: needs 2 to 8 stages, has 1",
		},
		{
			"unknown fails_at",
			fence(
				"docz-stepper",
				"steps:\n  - {key: a, name: A, does: x}\n  - {key: b, name: B, does: y}\nscenarios:\n  - {name: S, fails_at: c}\n",
			),
			`scenarios[0].fails_at: &#34;c&#34; is no step&#39;s key`,
		},
		{
			"missing score",
			fence(
				"docz-scorecard",
				"checks:\n  - {id: C1, name: N, target: T}\nexample:\n  results:\n    - {id: C1}\n",
			),
			"example.results[0].score: required",
		},
		{
			"unknown result key",
			fence(
				"docz-scorecard",
				"checks:\n  - {id: C1, name: N, target: T}\nexample:\n  results:\n    - {id: C1, score: 1, grade: A}\n",
			),
			"line 15: field grade not found",
		},
		{"empty", fence("docz-scorecard", ""), "line 10: the block is empty"},
		{"syntax", fence("docz-flow", "stages: [\n"), "line 11: did not find expected node content"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := page(t, tt.src, &RenderOptions{})
			body := string(p.Body)

			if islandOf.MatchString(body) {
				t.Fatalf("an invalid block is drawn")
			}

			if !strings.Contains(body, `<div class="callout caution"><p class="callout-title">Invalid docz-`) {
				t.Errorf("no caution callout")
			}

			if !strings.Contains(body, `<pre><code class="language-yaml">`) {
				t.Errorf("the source is not shown")
			}

			if !strings.Contains(body, tt.want) {
				t.Errorf("the page does not say %q", tt.want)
			}

			var found bool

			for _, e := range p.Problems {
				found = found || strings.Contains(e.Error(), stdhtml.UnescapeString(tt.want))
			}

			if !found {
				t.Errorf("Problems = %v, want one saying %q", p.Problems, tt.want)
			}
		})
	}
}
