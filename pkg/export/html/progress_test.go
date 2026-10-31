package html

import (
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/impl"
)

// phaseProgressSpan is a phase card's "done / total".
var phaseProgressSpan = regexp.MustCompile(`<span class="progress">(\d+) / (\d+)</span>`)

// TestPhaseProgressMatchesImpl holds every IMPL page's phase cards, summed,
// to impl.Doc.Progress over the same document.
func TestPhaseProgressMatchesImpl(t *testing.T) {
	t.Parallel()

	for path, typ := range corpus(t) {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}

		doc, err := impl.Parse(src)
		if err != nil {
			continue
		}

		t.Run(path, func(t *testing.T) {
			t.Parallel()

			var done, total int

			for _, m := range phaseProgressSpan.FindAllStringSubmatch(string(renderFile(t, path, typ)), -1) {
				d, _ := strconv.Atoi(m[1])
				n, _ := strconv.Atoi(m[2])
				done += d
				total += n
			}

			if wantDone, wantTotal := doc.Progress(); done != wantDone || total != wantTotal {
				t.Errorf("the page's phases count %d / %d, impl.Doc.Progress %d / %d", done, total, wantDone, wantTotal)
			}
		})
	}
}
