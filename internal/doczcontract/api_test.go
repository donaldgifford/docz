package doczcontract_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	doczcfg "github.com/donaldgifford/docz/pkg/doczcore/config"
	doczparse "github.com/donaldgifford/docz/pkg/doczcore/docparse"
)

// R10 — the docz v1.2.0 api surface (upstream DESIGN-0008 clause R10,
// DESIGN-0011): the api: config block's dormancy, load-time
// normalization, and enabled-only path validation behind the
// ErrInvalidAPIPath sentinel, plus docparse.Title. The DESIGN-0004 page
// pipeline (fetch hint, buildPages classifier, landing-page fallback)
// keys off every behavior pinned here, so a drift in any of them is a
// breaking change for docz-api even when the Go API still compiles.

// loadAPIConfig loads a .docz.yaml with the given body hermetically
// (same HOME-override pattern as loadChangelogConfig; a fresh temp repo
// dir per call so each case controls the full config).
func loadAPIConfig(t *testing.T, body string) doczcfg.Config {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, ".docz.yaml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write .docz.yaml: %v", err)
	}
	cfg, err := doczcfg.Load("", repo)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

func TestAPIConfigDefaults(t *testing.T) {
	// The fetch hint and buildPages both fall back to this constant when
	// composing <docs_dir>/index.md themselves.
	if doczcfg.APILandingFileName != "index.md" {
		t.Errorf("APILandingFileName = %q, want %q", doczcfg.APILandingFileName, "index.md")
	}

	api := doczcfg.DefaultConfig().API
	if api.Enabled {
		t.Error("DefaultConfig().API.Enabled = true, want false (opt-in)")
	}
	if api.LandingPage != "" {
		t.Errorf("DefaultConfig().API.LandingPage = %q, want empty (backfilled only when enabled)", api.LandingPage)
	}
	if len(api.Exclude) != 0 || len(api.AdditionalDocs) != 0 {
		t.Errorf("DefaultConfig().API lists = %v / %v, want both empty", api.Exclude, api.AdditionalDocs)
	}
}

func TestAPIConfigDormancy(t *testing.T) {
	// An absent block loads to the zero value — the byte-for-byte
	// today's-behavior guarantee for every repo that never opts in.
	cfg := loadAPIConfig(t, "docs_dir: docs\n")
	if cfg.API.Enabled || cfg.API.LandingPage != "" ||
		len(cfg.API.Exclude) != 0 || len(cfg.API.AdditionalDocs) != 0 {
		t.Errorf("absent block: API = %+v, want zero values", cfg.API)
	}
	if _, err := cfg.Validate(); err != nil {
		t.Errorf("Validate() = %v for absent block, want nil", err)
	}

	// A disabled block with hostile paths must load AND validate clean:
	// dormant paths are judged only at the moment they start being used,
	// so a repo can commit the block before any consumer understands it.
	const hostile = "api:\n" +
		"  enabled: false\n" +
		"  landing_page: ../escape.md\n" +
		"  exclude: [\"/abs\", \"../up\"]\n" +
		"  additional_docs: [\"/etc/passwd.md\", \"../../x.md\"]\n"
	cfg = loadAPIConfig(t, hostile)
	if _, err := cfg.Validate(); err != nil {
		t.Errorf("Validate() = %v for disabled block, want nil (unvalidated when dormant)", err)
	}
	// And the landing page is NOT backfilled while dormant.
	cfg = loadAPIConfig(t, "api:\n  enabled: false\n")
	if cfg.API.LandingPage != "" {
		t.Errorf("dormant LandingPage = %q, want empty (no backfill)", cfg.API.LandingPage)
	}
}

func TestAPIConfigNormalization(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want doczcfg.APIConfig
	}{
		{
			"leading dot-slash stripped",
			"api:\n  enabled: true\n  landing_page: ./docs/home.md\n  additional_docs: [./CONTRIBUTING.md]\n",
			doczcfg.APIConfig{Enabled: true, LandingPage: "docs/home.md", AdditionalDocs: []string{"CONTRIBUTING.md"}},
		},
		{
			"exclude trailing slash collapsed",
			"api:\n  enabled: true\n  exclude: [scratch/]\n",
			doczcfg.APIConfig{Enabled: true, LandingPage: "docs/index.md", Exclude: []string{"scratch"}},
		},
		{
			"empty landing page backfills default",
			"api:\n  enabled: true\n",
			doczcfg.APIConfig{Enabled: true, LandingPage: "docs/index.md"},
		},
		{
			"backfill tracks a non-default docs_dir",
			"docs_dir: notes\napi:\n  enabled: true\n",
			doczcfg.APIConfig{Enabled: true, LandingPage: "notes/index.md"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := loadAPIConfig(t, tt.yaml).API
			if api.LandingPage != tt.want.LandingPage {
				t.Errorf("LandingPage = %q, want %q", api.LandingPage, tt.want.LandingPage)
			}
			if !slices.Equal(api.Exclude, tt.want.Exclude) {
				t.Errorf("Exclude = %v, want %v", api.Exclude, tt.want.Exclude)
			}
			if !slices.Equal(api.AdditionalDocs, tt.want.AdditionalDocs) {
				t.Errorf("AdditionalDocs = %v, want %v", api.AdditionalDocs, tt.want.AdditionalDocs)
			}
		})
	}
}

// typesRFC is the explicit type block the reserved-segment cases enable,
// so the claim set under test is the fixture's own rather than the
// built-in defaults'.
const typesRFC = "types:\n" +
	"  rfc:\n" +
	"    enabled: true\n" +
	"    dir: rfc\n" +
	"    id_prefix: RFC\n" +
	"    id_width: 4\n" +
	"    statuses: [Draft]\n"

func TestAPIConfigValidateRejectsWhenEnabled(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			"traversal in landing page",
			"api:\n  enabled: true\n  landing_page: ../escape.md\n",
		},
		{
			"absolute additional doc",
			"api:\n  enabled: true\n  additional_docs: [\"/etc/passwd.md\"]\n",
		},
		{
			"win32 trailing period segment",
			"api:\n  enabled: true\n  landing_page: \"docs/index.md.\"\n",
		},
		{
			"additional doc under docs_dir",
			"api:\n  enabled: true\n  additional_docs: [docs/extra.md]\n",
		},
		{
			"reserved first segment",
			typesRFC + "api:\n  enabled: true\n  additional_docs: [rfc/notes.md]\n",
		},
		{
			// git stores "rfc%2Fnotes.md" as an ordinary filename, but a
			// router that decodes before matching sees the two-segment
			// form — the decoded spelling is checked too.
			"percent-encoded reserved separator",
			typesRFC + "api:\n  enabled: true\n  additional_docs: [\"rfc%2Fnotes.md\"]\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := loadAPIConfig(t, tt.yaml)
			_, err := cfg.Validate()
			if !errors.Is(err, doczcfg.ErrInvalidAPIPath) {
				t.Errorf("Validate() error = %v, want errors.Is ErrInvalidAPIPath", err)
			}
		})
	}
}

func TestDocparseTitle(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"atx with inline markdown stripped", "# **Bold** Title\n\nbody\n", "Bold Title"},
		{"setext h1", "My Title\n========\n\nbody\n", "My Title"},
		{"frontmatter skipped before h1", "---\ntitle: Frontmatter Title\n---\n\n# Real Title\n", "Real Title"},
		// The frontmatter title: key is NEVER read — "" is the normal
		// no-H1 outcome and the consumer supplies the filename fallback.
		{"frontmatter title ignored", "---\ntitle: Only In Frontmatter\n---\n\nprose only\n", ""},
		{"prose only", "just prose, no heading\n", ""},
		{"empty input", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := doczparse.Title([]byte(tt.content)); got != tt.want {
				t.Errorf("Title() = %q, want %q", got, tt.want)
			}
		})
	}
}
