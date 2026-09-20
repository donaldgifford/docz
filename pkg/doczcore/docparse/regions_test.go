package docparse

import (
	"strings"
	"testing"
)

func TestRole_String(t *testing.T) {
	t.Parallel()

	tests := map[Role]string{
		Start:   "start",
		End:     "end",
		Role(0): "unknown",
	}

	for role, want := range tests {
		if got := role.String(); got != want {
			t.Errorf("Role(%d).String() = %q, want %q", role, got, want)
		}
	}
}

func TestMarkers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want []Marker
	}{
		{
			name: "canonical pair",
			in:   "<!--docz:tasks:start-->\n- [ ] a\n<!--docz:tasks:end-->\n",
			want: []Marker{
				{Kind: "tasks", Role: Start, Line: 1, Canonical: true},
				{Kind: "tasks", Role: End, Line: 3, Canonical: true},
			},
		},
		{
			name: "hyphenated kind",
			in:   "<!--docz:file-changes:start-->\n",
			want: []Marker{{Kind: "file-changes", Role: Start, Line: 1, Canonical: true}},
		},
		{
			name: "indented marker is still canonical",
			in:   "  <!--docz:tasks:start-->\n",
			want: []Marker{{Kind: "tasks", Role: Start, Line: 1, Canonical: true}},
		},
		{
			name: "lenient inner whitespace is not canonical",
			in: "<!-- docz:tasks:start -->\n" +
				"<!--docz :tasks: end-->\n" +
				"<!--docz:tasks : start-->\n",
			want: []Marker{
				{Kind: "tasks", Role: Start, Line: 1, Canonical: false},
				{Kind: "tasks", Role: End, Line: 2, Canonical: false},
				{Kind: "tasks", Role: Start, Line: 3, Canonical: false},
			},
		},
		{
			name: "trailing text is not a marker",
			in:   "<!--docz:tasks:start--> and then\n",
			want: nil,
		},
		{
			name: "leading text is not a marker",
			in:   "see <!--docz:tasks:start-->\n",
			want: nil,
		},
		{
			name: "unknown role is not a marker",
			in:   "<!--docz:tasks:middle-->\n",
			want: nil,
		},
		{
			name: "uppercase kind is not a marker",
			in:   "<!--docz:Tasks:start-->\n",
			want: nil,
		},
		{
			name: "kind may not start with a digit",
			in:   "<!--docz:1tasks:start-->\n",
			want: nil,
		},
		{
			name: "other html comments are not markers",
			in:   "<!-- markdownlint-disable-file MD025 MD041 -->\n<!-- a note -->\n",
			want: nil,
		},
		{
			name: "markers inside a fence are text",
			in: "```markdown\n<!--docz:tasks:start-->\n```\n" +
				"<!--docz:tasks:start-->\n",
			want: []Marker{{Kind: "tasks", Role: Start, Line: 4, Canonical: true}},
		},
		{
			name: "tilde fence does not toggle",
			in:   "~~~\n<!--docz:tasks:start-->\n~~~\n",
			want: []Marker{{Kind: "tasks", Role: Start, Line: 2, Canonical: true}},
		},
		{
			name: "legacy toc pair reports kind toc",
			in:   "<!--toc:start-->\n- [A](#a)\n<!--toc:end-->\n",
			want: []Marker{
				{Kind: TocKind, Role: Start, Line: 1, Canonical: true},
				{Kind: TocKind, Role: End, Line: 3, Canonical: true},
			},
		},
		{
			name: "lenient toc spelling is flagged",
			in:   "<!-- toc:start -->\n",
			want: []Marker{{Kind: TocKind, Role: Start, Line: 1, Canonical: false}},
		},
		{
			name: "readme index pair reports kind index",
			in: "<!-- BEGIN DOCZ AUTO-GENERATED -->\n| ID |\n" +
				"<!-- END DOCZ AUTO-GENERATED -->\n",
			want: []Marker{
				{Kind: IndexKind, Role: Start, Line: 1, Canonical: true},
				{Kind: IndexKind, Role: End, Line: 3, Canonical: true},
			},
		},
		{
			name: "index pair without its spaces is flagged",
			in:   "<!--BEGIN DOCZ AUTO-GENERATED-->\n",
			want: []Marker{{Kind: IndexKind, Role: Start, Line: 1, Canonical: false}},
		},
		{
			// Refusing CRLF belongs to the write side; a reader that saw no
			// markers here would report the file as unstructured instead.
			name: "a trailing carriage return is trimmed with other whitespace",
			in:   "<!--docz:tasks:start-->\r\n",
			want: []Marker{{Kind: "tasks", Role: Start, Line: 1, Canonical: true}},
		},
		{
			name: "empty input",
			in:   "",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Markers([]byte(tt.in))

			if len(got) != len(tt.want) {
				t.Fatalf("Markers() = %+v, want %+v", got, tt.want)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Markers()[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestRegions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want []Region
	}{
		{
			name: "one closed region",
			in:   "<!--docz:summary:start-->\n## Summary\n<!--docz:summary:end-->\n",
			want: []Region{{Kind: "summary", Start: 1, End: 3, Depth: 0, Closed: true}},
		},
		{
			name: "nested regions carry depth",
			in: "<!--docz:phase:start-->\n### Phase 1: X\n" +
				"<!--docz:tasks:start-->\n- [ ] a\n<!--docz:tasks:end-->\n" +
				"<!--docz:criteria:start-->\n- ok\n<!--docz:criteria:end-->\n" +
				"<!--docz:phase:end-->\n",
			want: []Region{
				{Kind: "phase", Start: 1, End: 9, Depth: 0, Closed: true},
				{Kind: "tasks", Start: 3, End: 5, Depth: 1, Closed: true},
				{Kind: "criteria", Start: 6, End: 8, Depth: 1, Closed: true},
			},
		},
		{
			name: "three levels",
			in: "<!--docz:consequences:start-->\n<!--docz:positive:start-->\n" +
				"<!--docz:positive:end-->\n<!--docz:consequences:end-->\n",
			want: []Region{
				{Kind: "consequences", Start: 1, End: 4, Depth: 0, Closed: true},
				{Kind: "positive", Start: 2, End: 3, Depth: 1, Closed: true},
			},
		},
		{
			name: "a kind may repeat at the same depth",
			in: "<!--docz:phase:start-->\n<!--docz:phase:end-->\n" +
				"<!--docz:phase:start-->\n<!--docz:phase:end-->\n",
			want: []Region{
				{Kind: "phase", Start: 1, End: 2, Depth: 0, Closed: true},
				{Kind: "phase", Start: 3, End: 4, Depth: 0, Closed: true},
			},
		},
		{
			name: "end closes the innermost of its kind",
			in: "<!--docz:phase:start-->\n<!--docz:phase:start-->\n" +
				"<!--docz:phase:end-->\n<!--docz:phase:end-->\n",
			want: []Region{
				{Kind: "phase", Start: 1, End: 4, Depth: 0, Closed: true},
				{Kind: "phase", Start: 2, End: 3, Depth: 1, Closed: true},
			},
		},
		{
			name: "stray end yields no region",
			in:   "<!--docz:tasks:end-->\n## Body\n",
			want: nil,
		},
		{
			name: "unclosed at end of file",
			in:   "<!--docz:references:start-->\n## References\n",
			want: []Region{{Kind: "references", Start: 1, End: 3, Depth: 0, Closed: false}},
		},
		{
			name: "a parent's end cuts off an unclosed child",
			in: "<!--docz:scope:start-->\n<!--docz:in-scope:start-->\n" +
				"<!--docz:scope:end-->\n",
			want: []Region{
				{Kind: "scope", Start: 1, End: 3, Depth: 0, Closed: true},
				{Kind: "in-scope", Start: 2, End: 3, Depth: 1, Closed: false},
			},
		},
		{
			name: "legacy toc and index pairs are regions",
			in: "<!--toc:start-->\n<!--toc:end-->\n" +
				"<!-- BEGIN DOCZ AUTO-GENERATED -->\n<!-- END DOCZ AUTO-GENERATED -->\n",
			want: []Region{
				{Kind: TocKind, Start: 1, End: 2, Depth: 0, Closed: true},
				{Kind: IndexKind, Start: 3, End: 4, Depth: 0, Closed: true},
			},
		},
		{
			name: "lenient spellings still pair",
			in:   "<!-- docz:tasks:start -->\n<!--docz:tasks:end -->\n",
			want: []Region{{Kind: "tasks", Start: 1, End: 2, Depth: 0, Closed: true}},
		},
		{
			name: "no markers",
			in:   "# Title\n\nBody.\n",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Regions([]byte(tt.in))

			if len(got) != len(tt.want) {
				t.Fatalf("Regions() = %+v, want %+v", got, tt.want)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Regions()[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// Regions must come back in document order by start line, outermost first
// when two open on the same line, even though a parent is only resolved
// after its children.
func TestRegions_DocumentOrder(t *testing.T) {
	t.Parallel()

	in := "<!--docz:a:start-->\n<!--docz:b:start-->\n<!--docz:b:end-->\n" +
		"<!--docz:c:start-->\n<!--docz:c:end-->\n<!--docz:a:end-->\n"

	got := Regions([]byte(in))

	kinds := make([]string, 0, len(got))
	for _, r := range got {
		kinds = append(kinds, r.Kind)
	}

	if strings.Join(kinds, ",") != "a,b,c" {
		t.Errorf("order = %v, want [a b c]", kinds)
	}

	for i := 1; i < len(got); i++ {
		if got[i-1].Start > got[i].Start {
			t.Errorf("regions out of order at %d: %+v then %+v", i, got[i-1], got[i])
		}
	}
}

// Every region's markers must be lines Markers reports, so a writer can
// splice at either boundary.
func TestRegions_BoundariesAreMarkerLines(t *testing.T) {
	t.Parallel()

	in := "<!--docz:phase:start-->\n### Phase 1: X\n" +
		"<!--docz:tasks:start-->\n- [ ] a\n<!--docz:tasks:end-->\n<!--docz:phase:end-->\n"

	lines := make(map[int]bool)
	for _, m := range Markers([]byte(in)) {
		lines[m.Line] = true
	}

	for _, r := range Regions([]byte(in)) {
		if !lines[r.Start] {
			t.Errorf("region %s starts at line %d, which is not a marker line", r.Kind, r.Start)
		}

		if r.Closed && !lines[r.End] {
			t.Errorf("closed region %s ends at line %d, which is not a marker line", r.Kind, r.End)
		}
	}
}
