package index

import (
	"bytes"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/doctemplate"
)

// Splice is the whole of what this package does, as a pure function
// (IMPL-0018 Phase 2, DESIGN-0014 §2.6). These pin every outcome UpdateReadme
// and DryRunReadme can report, so the wrappers above can be rewritten without
// the behaviour moving.

const table = "| ID | Title |\n|----|-------|\n| RFC-0001 | A |\n"

func TestSplice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		existing []byte // nil means the README does not exist
		header   string
		want     string
		action   UpdateAction
	}{
		{
			name:     "no readme is created from the header",
			existing: nil,
			header:   "# RFCs\n\nProse.\n\n",
			want:     "# RFCs\n\nProse.\n\n" + BeginMarker + "\n" + table + EndMarker + "\n",
			action:   ActionCreated,
		},
		{
			name: "a header that already ends with a pair is not given a second",
			// Issue #99: index_investigation.md and index_plan.md end with
			// their own pair, and the creation path used to append another.
			existing: nil,
			header:   "# Investigations\n\nProse.\n\n" + BeginMarker + "\n" + EndMarker + "\n",
			want: "# Investigations\n\nProse.\n\n" +
				BeginMarker + "\n" + table + EndMarker + "\n",
			action: ActionCreated,
		},
		{
			name:     "an existing table is replaced between the markers",
			existing: []byte("# RFCs\n\n" + BeginMarker + "\n| stale |\n" + EndMarker + "\n"),
			header:   "ignored",
			want:     "# RFCs\n\n" + BeginMarker + "\n" + table + EndMarker + "\n",
			action:   ActionUpdated,
		},
		{
			name:     "an empty pair is filled",
			existing: []byte("# RFCs\n\n" + BeginMarker + "\n" + EndMarker + "\n"),
			header:   "ignored",
			want:     "# RFCs\n\n" + BeginMarker + "\n" + table + EndMarker + "\n",
			action:   ActionUpdated,
		},
		{
			name:     "content after the end marker is preserved",
			existing: []byte(BeginMarker + "\n" + EndMarker + "\n\n## Notes\n\nKeep me.\n"),
			want:     BeginMarker + "\n" + table + EndMarker + "\n\n## Notes\n\nKeep me.\n",
			action:   ActionUpdated,
		},
		{
			name:     "a file that ends on the end marker keeps ending there",
			existing: []byte("# RFCs\n" + BeginMarker + "\n" + EndMarker),
			want:     "# RFCs\n" + BeginMarker + "\n" + table + EndMarker,
			action:   ActionUpdated,
		},
		{
			name:     "a readme with no markers is left alone",
			existing: []byte("# RFCs\n\nSomebody wrote this by hand.\n"),
			header:   "ignored",
			want:     "",
			action:   ActionNoMarkers,
		},
		{
			name: "an unclosed pair is left alone",
			// Half a pair is not a region docz can splice into, and guessing
			// where it ends would rewrite content nobody marked.
			existing: []byte("# RFCs\n\n" + BeginMarker + "\n\nProse.\n"),
			want:     "",
			action:   ActionNoMarkers,
		},
		{
			name:     "an existing but empty readme is left alone",
			existing: []byte{},
			want:     "",
			action:   ActionNoMarkers,
		},
		{
			name: "the first pair wins when a readme carries two",
			// Issue #99's own output: the second pair sits empty forever, and
			// this is the behaviour that made it invisible.
			existing: []byte(BeginMarker + "\n| stale |\n" + EndMarker + "\n" +
				BeginMarker + "\n" + EndMarker + "\n"),
			want: BeginMarker + "\n" + table + EndMarker + "\n" +
				BeginMarker + "\n" + EndMarker + "\n",
			action: ActionUpdated,
		},
		{
			name: "a leniently spelled pair is found and normalised",
			// INV-0009 Finding 4: a marker with a stray space used to make
			// docz skip the file and say nothing at all.
			existing: []byte("# RFCs\n\n<!--  BEGIN DOCZ AUTO-GENERATED  -->\n" +
				"<!-- END DOCZ AUTO-GENERATED -->\n"),
			want:   "# RFCs\n\n" + BeginMarker + "\n" + table + EndMarker + "\n",
			action: ActionUpdated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, action := Splice(tt.existing, tt.header, table)

			if action != tt.action {
				t.Errorf("action = %v, want %v", action, tt.action)
			}

			if tt.action == ActionNoMarkers {
				if got != nil {
					t.Errorf("body = %q, want nil for ActionNoMarkers", got)
				}

				return
			}

			if string(got) != tt.want {
				t.Errorf("body mismatch\ngot:  %q\nwant: %q", got, tt.want)
			}
		})
	}
}

// TestSplice_DoesNotModifyItsInput pins that a consumer may keep or reuse the
// bytes it passed, the same contract the docwrite byte cores keep.
func TestSplice_DoesNotModifyItsInput(t *testing.T) {
	t.Parallel()

	original := "# RFCs\n\n" + BeginMarker + "\n| stale |\n" + EndMarker + "\n"
	existing := []byte(original)

	if _, action := Splice(existing, "", table); action != ActionUpdated {
		t.Fatalf("action = %v, want ActionUpdated", action)
	}

	if string(existing) != original {
		t.Error("Splice modified its input")
	}
}

// TestScaffold_ExactlyOnePairForEveryType is the issue #99 regression, over
// every header docz can resolve: the six built-ins through the embedded
// per-type tier, and a custom type through the rendered generic tier.
//
// The check is a count, not a string search, because the bug was not a wrong
// pair but a second one — and the splice only ever touches the first, so the
// extra sat empty in every new repo forever.
func TestScaffold_ExactlyOnePairForEveryType(t *testing.T) {
	t.Parallel()

	names := append(config.DocTypeNames(), "frameworks")

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			header, err := doctemplate.ResolveIndexHeader(name, t.TempDir(),
				doctemplate.IndexHeaderData{TypeName: name, PluralLabel: name})
			if err != nil {
				t.Fatalf("ResolveIndexHeader(%q): %v", name, err)
			}

			body := Scaffold(header)

			if got := bytes.Count(body, []byte(BeginMarker)); got != 1 {
				t.Errorf("%d begin markers, want exactly 1:\n%s", got, body)
			}

			if got := bytes.Count(body, []byte(EndMarker)); got != 1 {
				t.Errorf("%d end markers, want exactly 1:\n%s", got, body)
			}

			// And the scaffold is spliceable: a pair Splice cannot find would
			// make a created README never receive its table.
			spliced, action := Splice(nil, header, table)
			if action != ActionCreated {
				t.Fatalf("action = %v, want ActionCreated", action)
			}

			if !bytes.Contains(spliced, []byte(table)) {
				t.Errorf("the created README has no table:\n%s", spliced)
			}
		})
	}
}

// TestScaffold_TrailingPairDetection covers the header shapes the check has to
// tell apart, since the two that carry a pair are the only reason it exists.
func TestScaffold_TrailingPairDetection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
		want   bool
	}{
		{"no markers at all", "# RFCs\n\nProse.\n", false},
		{"pair at the end", "# X\n" + BeginMarker + "\n" + EndMarker + "\n", true},
		{
			name:   "pair followed by blank lines",
			header: "# X\n" + BeginMarker + "\n" + EndMarker + "\n\n\n",
			want:   true,
		},
		{
			name: "pair followed by prose is not trailing",
			// A header that ends with prose needs its own pair appended, or
			// the table would land above the prose.
			header: "# X\n" + BeginMarker + "\n" + EndMarker + "\n\n## Notes\n",
			want:   false,
		},
		{
			name:   "an unclosed begin marker is not a pair",
			header: "# X\n" + BeginMarker + "\n",
			want:   false,
		},
		{
			name:   "a stray end marker is not a pair",
			header: "# X\n" + EndMarker + "\n",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := hasTrailingPair(tt.header); got != tt.want {
				t.Errorf("hasTrailingPair = %v, want %v", got, tt.want)
			}

			// Whatever the answer, the scaffold holds exactly one pair.
			body := Scaffold(tt.header)

			begin := bytes.Count(body, []byte(BeginMarker))
			end := bytes.Count(body, []byte(EndMarker))

			if tt.want {
				if begin != 1 || end != 1 {
					t.Errorf("begin/end = %d/%d, want 1/1 (no pair appended)", begin, end)
				}

				return
			}

			// A header with a non-trailing pair legitimately ends up with two:
			// the one in its prose and the one appended. The appended pair is
			// the last thing in the file, which is what matters.
			if !strings.HasSuffix(string(body), BeginMarker+"\n"+EndMarker+"\n") {
				t.Errorf("scaffold does not end with a pair:\n%s", body)
			}
		})
	}
}
