package html

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// corpusFirst is the document line of a corpus block's first YAML line:
// each block is read as if its fence opened on line 1.
const corpusFirst = 2

// TestWidgetCorpus decodes every block in testdata/widgets and compares
// what it finds with the block's golden: the JSON island for a valid
// block, each error at its document line for an invalid one. docz-site's
// TypeScript validator reads the same files (IMPL-0030 Open Question 9),
// so a change on either side that the other lacks fails a test.
func TestWidgetCorpus(t *testing.T) {
	t.Parallel()

	blocks, err := filepath.Glob(filepath.Join("testdata", "widgets", "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	if len(blocks) == 0 {
		t.Fatal("testdata/widgets has no blocks")
	}

	for _, path := range blocks {
		base := strings.TrimSuffix(filepath.Base(path), ".yaml")

		t.Run(base, func(t *testing.T) {
			t.Parallel()

			widget, _, ok := strings.Cut(base, "-")
			if !ok || !isWidgetName(widget) {
				t.Fatalf("%s: name it <flow|stepper|scorecard>-<case>.yaml", path)
			}

			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			_, island, errs := decodeWidget(widget, body)
			parse := strings.HasPrefix(strings.TrimPrefix(base, widget+"-"), "parse-")

			switch {
			case errs == nil && parse:
				t.Fatalf("%s is named a parse case but is valid", path)
			case errs == nil:
				compare(t, strings.TrimSuffix(path, ".yaml")+".json.golden", append(island, '\n'))
			default:
				if parse && len(errs) != 1 {
					t.Errorf("a parse case has one error, got %q", errs)
				}

				lines := strings.Join(documentErrors(errs, corpusFirst), "\n") + "\n"
				compare(t, strings.TrimSuffix(path, ".yaml")+".errors.golden", []byte(lines))
			}
		})
	}
}

func isWidgetName(name string) bool {
	for _, short := range widgetNames {
		if short == name {
			return true
		}
	}

	return false
}
