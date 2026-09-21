package toc

import (
	"strings"
	"testing"
)

// Locating the ToC region through docparse.Regions rather than scanning
// for the literal marker text is what buys these three. Before it, a
// lenient spelling was invisible (INV-0009 Finding 4) and a fenced
// example of the markers was mistaken for the real thing.
func TestUpdateToC_ReadsTheRegionThroughTheWalker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		in        string
		wantFound bool
		wantHas   []string
		wantNot   []string
	}{
		{
			name: "a lenient spelling is found and canonicalized",
			in: "# T\n\n<!-- toc:start -->\n<!--  toc:end  -->\n\n" +
				"## Alpha\n\n## Beta\n\n## Gamma\n",
			wantFound: true,
			wantHas:   []string{BeginMarker + "\n", EndMarker, "- [Alpha](#alpha)"},
			wantNot:   []string{"<!-- toc:start -->", "<!--  toc:end  -->"},
		},
		{
			name: "markers shown inside a fence are not the region",
			in: "# T\n\n```markdown\n<!--toc:start-->\n<!--toc:end-->\n```\n\n" +
				"<!--toc:start-->\n<!--toc:end-->\n\n## Alpha\n\n## Beta\n\n## Gamma\n",
			wantFound: true,
			// The real region is the second pair, so the fenced example
			// survives and the headings below it are listed.
			wantHas: []string{"```markdown\n<!--toc:start-->", "- [Alpha](#alpha)"},
		},
		{
			name:      "an unclosed region is not a splice target",
			in:        "# T\n\n<!--toc:start-->\n\n## Alpha\n",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := UpdateToC(tt.in, 1)

			if got.Found != tt.wantFound {
				t.Fatalf("Found = %t, want %t\n%s", got.Found, tt.wantFound, got.Updated)
			}

			if !tt.wantFound {
				if got.Updated != tt.in {
					t.Errorf("input was modified:\n%s", got.Updated)
				}

				return
			}

			for _, want := range tt.wantHas {
				if !strings.Contains(got.Updated, want) {
					t.Errorf("output missing %q:\n%s", want, got.Updated)
				}
			}

			for _, unwanted := range tt.wantNot {
				if strings.Contains(got.Updated, unwanted) {
					t.Errorf("output still contains %q:\n%s", unwanted, got.Updated)
				}
			}
		})
	}
}
