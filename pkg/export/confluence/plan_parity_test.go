package confluence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
)

// corpusTypes maps each type package to the docz type its corpus belongs to.
var corpusTypes = []string{"rfc", "adr", "design", "impl", "investigation", "runbook"}

// corpusRepo builds a repository from the snapshotted fleet documents the
// type packages carry: each .orig.md becomes a numbered document of its
// package's type, the READMEs are generated, and the api: block adds a
// landing page and an additional doc. It reads the snapshots, never docs/,
// so the plan golden moves only when the planner does.
func corpusRepo(t *testing.T) *repo.Repo {
	t.Helper()

	cfg := config.DefaultConfig()
	rb := cfg.Types["runbook"]
	rb.Enabled = true
	cfg.Types["runbook"] = rb
	cfg.API = config.APIConfig{Enabled: true, LandingPage: "docs/index.md", AdditionalDocs: []string{"EXTRA.md"}}
	cfg.Sync.Confluence = config.ConfluenceSyncConfig{
		Enabled: true, Site: "https://example.atlassian.net", Space: "DOCZ", Parent: "docz",
		Layout: config.LayoutPage, APIPages: true,
		Mermaid: config.MermaidSyncConfig{Viewer: config.MermaidViewerAuto},
	}

	rp := &repo.Repo{Root: t.TempDir(), Cfg: &cfg}
	if _, err := rp.Init(t.Context(), repo.InitOptions{}); err != nil {
		t.Fatal(err)
	}

	for _, typeName := range corpusTypes {
		files, err := filepath.Glob(filepath.Join("..", "..", typeName, "testdata", "*.orig.md"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no %s corpus: %v", typeName, err)
		}

		for i, src := range files {
			b, err := os.ReadFile(src)
			if err != nil {
				t.Fatal(err)
			}

			name := fmt.Sprintf("%04d-%s.md", i+1, strings.TrimSuffix(filepath.Base(src), ".orig.md"))
			writeDoc(t, rp, filepath.Join(cfg.TypeDir(typeName), name), string(b))
		}
	}

	writeDoc(t, rp, "docs/index.md", "# Corpus\n\nSee [the first RFC](rfc/0001-booty-rfc-0001.md).\n")
	writeDoc(t, rp, "EXTRA.md", "# Extra\n\nAn additional doc.\n")

	if _, err := rp.Update(t.Context(), nil, repo.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	return rp
}

// planFacts is what the plan golden records: everything buildPlan decides,
// with each item's bytes reduced to a digest.
type planFacts struct {
	Full    bool                `json:"full"`
	Items   []itemFacts         `json:"items"`
	Keys    []string            `json:"keys"`
	Titles  []string            `json:"titles"`
	Targets map[string][]string `json:"targets"`
}

type itemFacts struct {
	Key      string `json:"key"`
	DocID    string `json:"doc_id,omitempty"`
	Title    string `json:"title"`
	Source   string `json:"source,omitempty"`
	Override string `json:"override,omitempty"`
	Parent   int    `json:"parent"`
	SHA      string `json:"sha256"`
}

func factsOf(p *plan) planFacts {
	f := planFacts{Full: p.full, Targets: make(map[string][]string, len(p.targets))}

	for i := range p.items {
		it := &p.items[i]
		sum := sha256.Sum256(it.src)
		f.Items = append(f.Items, itemFacts{
			Key: it.key, DocID: it.docID, Title: it.title, Source: it.source,
			Override: it.override, Parent: it.parent, SHA: hex.EncodeToString(sum[:8]),
		})
	}

	for k := range p.keys {
		f.Keys = append(f.Keys, k)
	}

	for k := range p.titles {
		f.Titles = append(f.Titles, k)
	}

	slices.Sort(f.Keys)
	slices.Sort(f.Titles)

	for src, tg := range p.targets {
		anchors := make([]string, 0, len(tg.headings)+1)
		anchors = append(anchors, tg.title)

		for slug := range tg.headings {
			anchors = append(anchors, slug)
		}

		slices.Sort(anchors[1:])
		f.Targets[src] = anchors
	}

	return f
}

// TestPlan_Golden pins everything buildPlan decides over the corpus
// repository, for a full run and two narrowed ones (IMPL-0024 Phase 2),
// read through os.DirFS.
func TestPlan_Golden(t *testing.T) {
	t.Parallel()

	checkPlanGolden(t, corpusRepo(t), nil)
}

// TestPlanFromMapFS proves the plan is the same when the files come from
// memory: the corpus repository copied into an fstest.MapFS.
func TestPlanFromMapFS(t *testing.T) {
	t.Parallel()

	rp := corpusRepo(t)

	checkPlanGolden(t, rp, mapFS(t, rp.Root))
}

// mapFS copies every file under root into an fstest.MapFS.
func mapFS(t *testing.T, root string) fstest.MapFS {
	t.Helper()

	m := fstest.MapFS{}

	err := fs.WalkDir(os.DirFS(root), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		if err != nil {
			return err
		}

		m[p] = &fstest.MapFile{Data: b}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	return m
}

// checkPlanGolden plans rp through fsys (nil for os.DirFS) for each golden
// case and compares the facts, or rewrites them under -update.
func checkPlanGolden(t *testing.T, rp *repo.Repo, fsys fs.FS) {
	t.Helper()

	for name, opts := range map[string]ExportOptions{
		"full":  {},
		"types": {Types: []string{"adr", "inv"}},
		"ids":   {IDs: []string{"RFC-0002", "DESIGN-0014"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			opts.FS = fsys

			p, err := buildPlan(t.Context(), rp, &opts)
			if err != nil {
				t.Fatal(err)
			}

			got, err := json.MarshalIndent(factsOf(p), "", "  ")
			if err != nil {
				t.Fatal(err)
			}

			golden := path.Join("testdata", "plan", name+".golden.json")
			if *update && fsys == nil {
				if err := os.MkdirAll(filepath.Dir(golden), 0o750); err != nil {
					t.Fatal(err)
				}

				if err := os.WriteFile(golden, append(got, '\n'), 0o600); err != nil {
					t.Fatal(err)
				}

				return
			}

			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}

			if string(want) != string(got)+"\n" {
				t.Errorf("plan differs from %s; run with -update only if the change is intended", golden)
			}
		})
	}
}
