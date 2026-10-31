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
			name: "regions in order, ToC dropped",
			src: "# T\n\n<!--toc:start-->\n- [A](#a)\n<!--toc:end-->\n\n" +
				"<!--docz:summary:start-->\n## Summary\n\nx\n<!--docz:summary:end-->\n\n" +
				"<!--docz:problem:start-->\n## Problem Statement\n\ny\n<!--docz:problem:end-->\n",
			want: "summary=Summary problem-statement=Problem Statement",
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
			want: "summary=Summary extra=Extra",
		},
		{
			name: "a region with no heading is labelled by its kind",
			src:  "<!--docz:non-goals:start-->\n- a\n<!--docz:non-goals:end-->\n",
			want: "non-goals=Non goals",
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
