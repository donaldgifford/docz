package toc_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/toc"
)

// fragmentPattern captures the fragment of an in-page link.
var fragmentPattern = regexp.MustCompile(`\]\(#([^)]*)\)`)

// TestAnchorsMatchGitHub checks docz's ToC for docparse's anchors.md
// against the link written by hand under each heading. markdownlint's
// MD051 accepts every one of those links (`just lint-generated` lints the
// file), so the two together tie the slug to an external ground truth.
func TestAnchorsMatchGitHub(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("../docparse/testdata/anchors.md")
	if err != nil {
		t.Fatal(err)
	}

	var want []string
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "## ") || i+2 >= len(lines) {
			continue
		}
		if m := fragmentPattern.FindStringSubmatch(lines[i+2]); m != nil {
			want = append(want, m[1])
		}
	}

	generated := toc.GenerateToC(docparse.Headings(content), 0)
	matches := fragmentPattern.FindAllStringSubmatch(generated, -1)
	got := make([]string, 0, len(matches))
	for _, m := range matches {
		got = append(got, m[1])
	}

	if len(want) == 0 {
		t.Fatal("anchors.md has no hand-written links")
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("ToC fragments differ from anchors.md\n got: %q\nwant: %q", got, want)
	}
}
