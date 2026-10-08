package confluence

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
)

var update = flag.Bool("update", false, "update golden files")

// localIDValue matches a minted viewer local id, so goldens and the
// invariant tests compare pages with the random part restored.
var localIDValue = regexp.MustCompile(`(key="local-id">)[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}(<)`)

// restoreLocalIDs puts a fixed value back where Render minted ids.
func restoreLocalIDs(body []byte) []byte {
	return localIDValue.ReplaceAll(body, []byte("${1}LOCAL-ID${2}"))
}

// testResolver places the links the goldens use: one exported document, a
// source file, and a local image; everything else is unresolved.
func testResolver(from, href string) LinkTarget {
	path, frag, _ := strings.Cut(href, "#")

	switch path {
	case "../impl/0001-plan.md":
		t := LinkTarget{PageTitle: "IMPL-0001: Plan"}
		if frag != "" {
			t.Anchor = anchorID(t.PageTitle, "Phase 1")
		}

		return t
	case "../../cmd/root.go":
		return LinkTarget{URL: "https://github.com/o/r/blob/main/cmd/root.go"}
	case "diagram.png":
		return LinkTarget{URL: "https://github.com/o/r/blob/main/" + filepath.Dir(from) + "/diagram.png"}
	}

	return LinkTarget{}
}

// goldenOptions is the RenderOptions each golden case renders with.
func goldenOptions(name string) RenderOptions {
	opts := RenderOptions{Resolve: testResolver, Source: "docs/design/0001-spec.md"}

	switch name {
	case "banner":
		opts.SourceURL = "https://github.com/o/r/blob/main/docs/design/0001-spec.md"
	case "banner-nourl":
	case "banner-overwrite":
		opts.SourceURL = "https://github.com/o/r/blob/main/docs/design/0001-spec.md"
		opts.Overwrite = true
	case "mermaid-code":
		opts.Mermaid = MermaidCode
		opts.Source = ""
	default:
		opts.Source = ""
	}

	return opts
}

func TestRender_Goldens(t *testing.T) {
	t.Parallel()

	inputs, err := filepath.Glob(filepath.Join("testdata", "render", "*.md"))
	if err != nil || len(inputs) == 0 {
		t.Fatalf("no golden inputs: %v", err)
	}

	for _, in := range inputs {
		name := strings.TrimSuffix(filepath.Base(in), ".md")

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			src, err := os.ReadFile(in)
			if err != nil {
				t.Fatal(err)
			}

			got, err := Render(src, goldenOptions(name))
			if err != nil {
				t.Fatalf("Render: %v", err)
			}

			var b bytes.Buffer
			b.WriteString("title: " + got.Title + "\n")
			for _, l := range got.Links {
				b.WriteString("unresolved: " + l.Href + " (" + l.Text + ") line " + strconv.Itoa(l.Line) + "\n")
			}
			b.WriteString("---\n")
			b.Write(restoreLocalIDs(got.Body))

			golden := strings.TrimSuffix(in, ".md") + ".xhtml"
			if *update {
				if err := os.WriteFile(golden, b.Bytes(), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden (run with -update): %v", err)
			}

			if !bytes.Equal(b.Bytes(), want) {
				t.Errorf("Render(%s) mismatch\ngot:\n%s\nwant:\n%s", name, b.Bytes(), want)
			}
		})
	}
}

func TestRender_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		opts RenderOptions
		want error
	}{
		{"CR line endings", "---\r\nid: X\r\n---\r\n", RenderOptions{}, document.ErrUnsupportedLineEndings},
		{"no frontmatter, no title", "# Just a heading\n", RenderOptions{}, document.ErrNoFrontmatter},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := Render([]byte(tt.src), tt.opts); !errors.Is(err, tt.want) {
				t.Errorf("Render() = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestRender_TitleOverrideAllowsNoFrontmatter(t *testing.T) {
	t.Parallel()

	got, err := Render([]byte("# Design documents\n\n| a |\n| - |\n| b |\n"), RenderOptions{Title: "Design"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if got.Title != "Design" || got.ID != "" {
		t.Errorf("Title, ID = %q, %q, want Design and empty", got.Title, got.ID)
	}
}

func TestRender_MalformedOutputIsAnError(t *testing.T) {
	t.Parallel()

	// An allowed tag left open passes the allow-list and breaks the XML.
	src := "---\nid: X-1\ntitle: T\n---\n\nText <span>never closed.\n"

	_, err := Render([]byte(src), RenderOptions{})

	var malformed *MalformedError
	if !errors.As(err, &malformed) || malformed.Line == 0 {
		t.Fatalf("Render() = %v, want *MalformedError with a line", err)
	}
}
