package html

import (
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

// labels outlines src and returns "key=label" per tab.
func labels(t *testing.T, src string) []string {
	t.Helper()

	body := blankFrontmatter([]byte("---\nid: X-0001\ntitle: T\n---\n" + src))
	regions, _ := kinds.ResolveRegions(body, nil)
	tabs := outline(strings.Split(string(body), "\n"), regions, docparse.Headings(body))

	out := make([]string, 0, len(tabs))
	for _, tb := range tabs {
		out = append(out, tb.Key+"="+tb.Label)
	}

	return out
}

func TestOutline(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "regions go to their row's tab, ToC dropped",
			src: "# T\n\n<!--toc:start-->\n- [A](#a)\n<!--toc:end-->\n\n" +
				"<!--docz:summary:start-->\n## Summary\n\nx\n<!--docz:summary:end-->\n\n" +
				"<!--docz:problem:start-->\n## Problem Statement\n\ny\n<!--docz:problem:end-->\n",
			want: "overview=Overview problem=Problem",
		},
		{
			name: "no regions: one tab per H2",
			src:  "# T\n\nintro\n\n## One\n\na\n\n## Two\n\nb\n",
			want: "one=One two=Two",
		},
		{
			name: "an H2 outside every region is a tab of its own",
			src: "<!--docz:summary:start-->\n## Summary\n\nx\n<!--docz:summary:end-->\n\n" +
				"## Extra\n\ny\n",
			want: "overview=Overview extra=Extra",
		},
		{
			name: "an unknown kind with no heading is labelled by its kind",
			src:  "<!--docz:threat-model:start-->\n- a\n<!--docz:threat-model:end-->\n",
			want: "threat-model=Threat model",
		},
		{
			name: "an unknown kind is a tab named by its heading",
			src: "<!--docz:summary:start-->\n## Summary\n\nx\n<!--docz:summary:end-->\n\n" +
				"<!--docz:threat-model:start-->\n## Threats\n\ny\n<!--docz:threat-model:end-->\n",
			want: "overview=Overview threats=Threats",
		},
		{
			name: "regions sharing a tab join it where its first region is",
			src: "<!--docz:summary:start-->\n## Summary\n\nx\n<!--docz:summary:end-->\n\n" +
				"<!--docz:proposal:start-->\n## Proposal\n\ny\n<!--docz:proposal:end-->\n\n" +
				"<!--docz:goals:start-->\n## Goals\n\n- g\n<!--docz:goals:end-->\n",
			want: "overview=Overview proposal=Proposal",
		},
		{
			name: "a heading that only introduces regions joins the next tab",
			src: "## Goals and Non-Goals\n\n<!--docz:goals:start-->\n### Goals\n\n- a\n<!--docz:goals:end-->\n\n" +
				"<!--docz:non-goals:start-->\n### Non-Goals\n\n- b\n<!--docz:non-goals:end-->\n",
			want: "overview=Overview",
		},
		{
			name: "an H2 with a paragraph over deeper regions opens their tab",
			src: "## Implementation Phases\n\nEach phase builds on the last.\n\n" +
				"<!--docz:phase:start-->\n### Phase 1: A\n\n- [ ] t\n<!--docz:phase:end-->\n\n" +
				"<!--docz:references:start-->\n## References\n\n- r\n<!--docz:references:end-->\n",
			want: "phases=Phases references=References",
		},
		{
			name: "an H2 with a paragraph over a peer region keeps its tab",
			src: "## Notes\n\nA note.\n\n" +
				"<!--docz:references:start-->\n## References\n\n- r\n<!--docz:references:end-->\n",
			want: "notes=Notes references=References",
		},
		{
			name: "duplicate labels get distinct keys",
			src:  "## Notes\n\na\n\n## Notes\n\nb\n",
			want: "notes=Notes notes-2=Notes",
		},
		{
			name: "only a title: one document tab",
			src:  "# T\n\njust text\n",
			want: "document=Document",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := strings.Join(labels(t, tt.src), " "); got != tt.want {
				t.Errorf("outline = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestContents(t *testing.T) {
	t.Parallel()

	src := "## Design\n\n### A\n\n### B\n\n#### B.1\n\n### C\n\n### D\n\n### E\n\n## Few\n\n### X\n"
	_, d := renderBody(t, src, &RenderOptions{})

	panels, err := d.panels()
	if err != nil {
		t.Fatal(err)
	}

	got := make([]string, 0, len(panels[0].Contents))
	for _, h := range panels[0].Contents {
		got = append(got, h.Slug)
	}

	if strings.Join(got, " ") != "a b c d e" {
		t.Errorf("Design contents = %v, want the five H3s", got)
	}

	if panels[0].Anchor != "design" {
		t.Errorf("Design title anchor = %q, want design", panels[0].Anchor)
	}

	if len(panels[1].Contents) != 0 {
		t.Errorf("Few contents = %v, want none for one heading", panels[1].Contents)
	}
}
