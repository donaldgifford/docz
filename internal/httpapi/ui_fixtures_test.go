package httpapi

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/donaldgifford/docz/v2/internal/store"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
)

var update = flag.Bool("update", false, "regenerate ui/src/mocks/__generated__/fragments.json")

// uiFixture is one of docz-site's MSW demo documents
// (ui/src/mocks/fixtures.ts DEMO_DOCS): its repository, type, path, and
// the content file it is read from.
type uiFixture struct{ repo, typ, path, file string }

var uiFixtures = []uiFixture{
	{"docz-site", "design", "docs/design/0001-docz-site-cross-repo-docz-reader-and-search-ui.md", "docz-site-design-0001.md"},
	{"docz-site", "impl", "docs/impl/0001-docz-site-mvp-phased-build-of-the-reader-directory-and-repo.md", "docz-site-impl-0001.md"},
	{"docz-site", "runbook", "docs/runbook/0001-cut-a-v2-beta-release.md", "docz-site-runbook-0001.md"},
	{"docz-api", "design", "docs/design/0001-docz-api-cross-repo-docz-registry-and-ingestion-service.md", "docz-api-design-0001.md"},
	{"docz-api", "design", "docs/design/0002-openapi-contract-for-docz-api-and-the-docz-site.md", "docz-api-design-0002.md"},
	{"docz-api", "rfc", "docs/rfc/0001-relative-doc-links-resolve-in-rendered-bodies.md", "docz-api-rfc-0001.md"},
}

// uiTypes are the demo repositories' types, as DEMO_TYPES declares them.
var uiTypes = []store.DocType{
	{Name: "rfc", Dir: "rfc", IDPrefix: "RFC"},
	{Name: "adr", Dir: "adr", IDPrefix: "ADR"},
	{Name: "design", Dir: "design", IDPrefix: "DESIGN"},
	{Name: "impl", Dir: "impl", IDPrefix: "IMPL"},
	{Name: "investigation", Dir: "investigation", IDPrefix: "INV"},
	{Name: "runbook", Dir: "runbook", IDPrefix: "RUNBOOK"},
}

// TestUIFragmentFixtures renders docz-site's demo documents through the
// handler's own renderFragment into the JSON its MSW mode serves, so the
// site's dev and e2e runs embed exactly what docz-api would send
// (IMPL-0032 Phase 2). Regenerate with go test -update.
func TestUIFragmentFixtures(t *testing.T) {
	t.Parallel()

	content := filepath.Join("..", "..", "ui", "src", "mocks", "content")
	out := make(map[string]DocumentHTMLDTO, len(uiFixtures))

	for _, f := range uiFixtures {
		raw, err := os.ReadFile(filepath.Join(content, f.file))
		if err != nil {
			t.Fatal(err)
		}

		fm, err := document.ParseFrontmatter(raw)
		if err != nil {
			t.Fatalf("%s: %v", f.file, err)
		}

		repo := &store.Repo{
			Owner: "donaldgifford", Name: f.repo, DocsDir: "docs",
			ConfigSnapshot: json.RawMessage(`{"docs_dir":"docs"}`),
		}
		doc := &store.Document{Type: f.typ, DocID: fm.ID, Path: f.path, RawMd: string(raw)}

		dto, err := renderFragment(repo, doc, uiTypes, nil)
		if err != nil {
			t.Fatalf("%s: %v", f.file, err)
		}

		out["donaldgifford/"+f.repo+"/"+fm.ID] = dto
	}

	got, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	got = append(got, '\n')
	golden := filepath.Join("..", "..", "ui", "src", "mocks", "__generated__", "fragments.json")

	if *update {
		if err := os.MkdirAll(filepath.Dir(golden), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(golden, got, 0o600); err != nil {
			t.Fatal(err)
		}

		return
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run go test ./internal/httpapi/ -update)", err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("%s is stale; run go test ./internal/httpapi/ -run TestUIFragmentFixtures -update", golden)
	}
}

// TestUIHostileFragments renders the UI's XSS payloads
// (ui/src/mocks/content/xss-payloads.json), each as the body of a
// document, through renderFragment, into the JSON the site's
// doc-pane.xss.test.tsx mounts: the fragment path's XSS gate runs over
// what docz-api really sends (IMPL-0032 Phase 4). Regenerate with
// go test -update.
func TestUIHostileFragments(t *testing.T) {
	t.Parallel()

	mocks := filepath.Join("..", "..", "ui", "src", "mocks")

	raw, err := os.ReadFile(filepath.Join(mocks, "content", "xss-payloads.json"))
	if err != nil {
		t.Fatal(err)
	}

	var payloads [][2]string
	if err := json.Unmarshal(raw, &payloads); err != nil {
		t.Fatal(err)
	}

	repo := &store.Repo{Owner: "acme", Name: "hostile", DocsDir: "docs", ConfigSnapshot: json.RawMessage(`{}`)}
	out := make(map[string]string, len(payloads))

	for i, p := range payloads {
		md := "---\nid: RFC-0001\ntitle: Hostile\nstatus: Draft\ncreated: 2026-03-04\n---\n\n# RFC-0001: Hostile\n\n## Summary\n\n" + p[1] + "\n"
		doc := &store.Document{Type: "rfc", DocID: "RFC-0001", Path: "docs/rfc/0001-hostile.md", RawMd: md}

		dto, err := renderFragment(repo, doc, uiTypes, nil)
		if err != nil {
			t.Fatalf("payload %d (%s): %v", i, p[0], err)
		}

		out[p[0]] = dto.HTML
	}

	got, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	got = append(got, '\n')
	golden := filepath.Join(mocks, "__generated__", "hostile-fragments.json")

	if *update {
		if err := os.WriteFile(golden, got, 0o600); err != nil {
			t.Fatal(err)
		}

		return
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run go test ./internal/httpapi/ -update)", err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("%s is stale; run go test ./internal/httpapi/ -run TestUIHostileFragments -update", golden)
	}
}
