package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// reviewFixtureEntry is one document in ui/e2e/fixtures/review-session.json,
// the capture the review page's dev server and e2e spec serve.
type reviewFixtureEntry struct {
	Inspect  map[string]any `json:"inspect"`
	Findings []any          `json:"findings"`
}

// TestReviewFixtureMatchesDocz fails when the review page's committed
// capture no longer matches what docz prints for the fixture repository,
// so the page is never tested against output docz stopped producing. The
// fix is `just build && bun run --cwd ui gen-review-fixture`.
func TestReviewFixtureMatchesDocz(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "ui", "e2e", "fixtures", "review-session.json"))
	if err != nil {
		t.Fatalf("reading the capture: %v", err)
	}

	var entries []reviewFixtureEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("decoding the capture: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("the capture holds no documents")
	}

	r, out, _ := copyRunner(t, filepath.Join("..", "ui", "e2e", "fixtures", "review-repo"))

	for _, want := range entries {
		path, ok := want.Inspect["path"].(string)
		if !ok {
			t.Fatalf("a captured document has no path: %v", want.Inspect)
		}

		t.Run(path, func(t *testing.T) {
			out.Reset()

			if err := r.inspect(t.Context(), formatJSON, path); err != nil {
				t.Fatalf("inspect(%q) error = %v", path, err)
			}

			var inspect map[string]any
			if err := json.Unmarshal(out.Bytes(), &inspect); err != nil {
				t.Fatalf("inspect(%q) output is not JSON: %v", path, err)
			}

			inspect["docz_version"] = want.Inspect["docz_version"]
			if !reflect.DeepEqual(inspect, want.Inspect) {
				t.Errorf("inspect(%q) differs from the capture; regenerate it with gen-review-fixture", path)
			}

			out.Reset()
			// Findings exit non-zero; the report is on out either way.
			_ = r.validate(t.Context(), validateOpts{format: formatJSON}, []string{path})

			var report struct {
				Docs []struct {
					Findings []any `json:"findings"`
				} `json:"docs"`
			}
			if err := json.Unmarshal(out.Bytes(), &report); err != nil {
				t.Fatalf("validate(%q) output is not JSON: %v", path, err)
			}

			var got []any
			if len(report.Docs) == 1 {
				got = report.Docs[0].Findings
			}

			if len(got) == 0 && len(want.Findings) == 0 {
				return
			}

			if !reflect.DeepEqual(got, want.Findings) {
				t.Errorf("validate(%q) findings differ from the capture; regenerate it with gen-review-fixture", path)
			}
		})
	}
}
