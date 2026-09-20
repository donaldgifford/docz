package kinds_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
)

var update = flag.Bool("update", false, "update golden files")

// The fixtures are verbatim cuts of real sections from this repo's own
// documents, so a golden records what a reader makes of the fleet's actual
// writing rather than of prose invented to suit it. That is the difference
// between a test that passes and evidence: every tolerance in these readers
// exists because a real document needed it.
//
// Regenerate with `go test ./pkg/doczcore/kinds/... -update`.
var readers = map[string]func([]byte) string{
	"openquestions": formatQuestions,
	"references":    formatReferences,
	"decisions":     formatDecisions,
	"criteria":      formatCriteria,
	"alternatives":  formatAlternatives,
}

func TestGoldenReaders(t *testing.T) {
	t.Parallel()

	for dir, format := range readers {
		t.Run(dir, func(t *testing.T) {
			t.Parallel()

			names, err := filepath.Glob(filepath.Join("testdata", dir, "*.md"))
			if err != nil {
				t.Fatal(err)
			}

			if len(names) == 0 {
				t.Fatalf("no fixtures under testdata/%s: the reader is untested against the corpus", dir)
			}

			for _, path := range names {
				name := strings.TrimSuffix(filepath.Base(path), ".md")

				t.Run(name, func(t *testing.T) {
					t.Parallel()

					content, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}

					compareFacts(t, filepath.Join("testdata", dir, "golden", name+".golden.txt"),
						format(content))
				})
			}
		})
	}
}

func compareFacts(t *testing.T, goldenPath, got string) {
	t.Helper()

	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}

		t.Log("updated golden file:", goldenPath)

		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden %s: %v\nRun with -update to create it", goldenPath, err)
	}

	if got != string(want) {
		t.Errorf("facts differ from %s\n--- got ---\n%s\n--- want ---\n%s", goldenPath, got, want)
	}
}

func formatQuestions(content []byte) string {
	var sb strings.Builder

	for _, q := range kinds.OpenQuestions(content) {
		fmt.Fprintf(&sb, "question %d line=%d title=%q\n", q.Number, q.Line, q.Title)

		if q.Resolved != nil {
			fmt.Fprintf(&sb, "  resolved line=%d date=%q choice=%q note=%q\n",
				q.Resolved.Line, q.Resolved.Date, q.Resolved.Choice, q.Resolved.Note)
		}

		for _, o := range q.Options {
			fmt.Fprintf(&sb, "  option %s line=%d recommended=%t text=%q\n",
				o.Letter, o.Line, o.Recommended, o.Text)
		}
	}

	return sb.String()
}

func formatReferences(content []byte) string {
	var sb strings.Builder

	for _, r := range kinds.References(content) {
		fmt.Fprintf(&sb, "line=%d url=%q text=%q\n", r.Line, r.URL, r.Text)
	}

	return sb.String()
}

func formatDecisions(content []byte) string {
	var sb strings.Builder

	for _, d := range kinds.Decisions(content) {
		fmt.Fprintf(&sb, "line=%d number=%d question=%q resolution=%q\n",
			d.Line, d.Number, d.Question, d.Resolution)
	}

	return sb.String()
}

func formatCriteria(content []byte) string {
	var sb strings.Builder

	for _, c := range kinds.Criteria(content) {
		fmt.Fprintf(&sb, "line=%d executable=%t command=%q text=%q\n",
			c.Line, c.Executable, c.Command, c.Text)
	}

	return sb.String()
}

func formatAlternatives(content []byte) string {
	var sb strings.Builder

	for _, a := range kinds.Alternatives(content) {
		fmt.Fprintf(&sb, "line=%d label=%q title=%q text=%q\n", a.Line, a.Label, a.Title, a.Text)
	}

	return sb.String()
}
