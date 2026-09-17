package consumer

// The v1.2.0 surface proof (IMPL-0016 Phase 3): the api: config block
// and docparse.Title exercised from outside the module, in the shape
// docz-api will freeze as contract clause R10.

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/donaldgifford/docz/pkg/doczcore/config"
	"github.com/donaldgifford/docz/pkg/doczcore/docparse"
)

const apiYAML = `docs_dir: docs
api:
  enabled: true
  landing_page: ./docs/index.md
  exclude:
    - ./scratch/
  additional_docs:
    - ./CONTRIBUTING.md
    - DEVELOPMENT.md
`

// TestExternalConsumerResolvesAPIConfig covers the config half of R10:
// the yaml keys, the enabled flag, and the load-time normalization a
// consumer relies on when it resolves these paths against a git tree it
// fetched without a checkout.
func TestExternalConsumerResolvesAPIConfig(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".docz.yaml"), apiYAML)

	cfg, err := config.Load("", root)
	if err != nil {
		t.Fatalf("config.Load(\"\", %q) = %v, want nil", root, err)
	}
	if _, err := cfg.Validate(); err != nil {
		t.Fatalf("cfg.Validate() = %v, want nil", err)
	}

	if !cfg.API.Enabled {
		t.Error("API.Enabled = false, want true")
	}
	if want := "docs/index.md"; cfg.API.LandingPage != want {
		t.Errorf("API.LandingPage = %q, want %q (leading ./ normalized away)",
			cfg.API.LandingPage, want)
	}

	// One stored spelling per exclude entry: a consumer matches this
	// deny-list by path prefix, so "scratch/" arriving verbatim would be
	// compared as "scratch//…" and exclude nothing.
	if got := cfg.API.Exclude; len(got) != 1 || got[0] != "scratch" {
		t.Errorf("API.Exclude = %q, want [scratch] (./ and trailing / normalized away)", got)
	}

	want := []string{"CONTRIBUTING.md", "DEVELOPMENT.md"}
	got := cfg.API.AdditionalDocs
	if len(got) != len(want) {
		t.Fatalf("API.AdditionalDocs = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("API.AdditionalDocs[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestExternalConsumerReadsAPIDefaults pins that a repo with no api
// block still resolves the documented defaults, so a consumer can read
// the fields unconditionally rather than testing for presence.
func TestExternalConsumerReadsAPIDefaults(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".docz.yaml"), "docs_dir: docs\n")

	cfg, err := config.Load("", root)
	if err != nil {
		t.Fatalf("config.Load on a block-less repo = %v, want nil", err)
	}
	if cfg.API.Enabled {
		t.Error("default API.Enabled = true, want false")
	}
	if cfg.API.LandingPage != "" {
		t.Errorf("default API.LandingPage = %q, want empty", cfg.API.LandingPage)
	}
	if cfg.API.Exclude != nil || cfg.API.AdditionalDocs != nil {
		t.Errorf("default API lists = %q / %q, want nil",
			cfg.API.Exclude, cfg.API.AdditionalDocs)
	}

	// The landing page is backfilled under docs_dir, so a consumer that
	// enables the block never has to construct the path itself.
	enabled := t.TempDir()
	writeFile(t, filepath.Join(enabled, ".docz.yaml"),
		"docs_dir: documentation\napi:\n  enabled: true\n")
	cfg, err = config.Load("", enabled)
	if err != nil {
		t.Fatalf("config.Load() = %v, want nil", err)
	}
	if want := "documentation/index.md"; cfg.API.LandingPage != want {
		t.Errorf("API.LandingPage = %q, want %q backfilled under docs_dir",
			cfg.API.LandingPage, want)
	}
}

// TestExternalConsumerDetectsInvalidAPIPath pins the sentinel: a
// consumer distinguishes "this repo's api paths are unsafe to fetch"
// from any other config failure with errors.Is, never by matching text.
func TestExternalConsumerDetectsInvalidAPIPath(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".docz.yaml"),
		"api:\n  enabled: true\n  additional_docs:\n    - ../../etc/passwd\n")

	cfg, err := config.Load("", root)
	if err != nil {
		t.Fatalf("config.Load() = %v, want nil: Load normalizes, Validate judges", err)
	}
	if _, err := cfg.Validate(); !errors.Is(err, config.ErrInvalidAPIPath) {
		t.Errorf("cfg.Validate() = %v, want it to wrap ErrInvalidAPIPath", err)
	}
}

// TestExternalConsumerToleratesDormantAPIBlock is the dormancy
// guarantee, and the reason a repo can commit the block before every
// consumer understands it: a disabled block never fails load, whatever
// it holds.
func TestExternalConsumerToleratesDormantAPIBlock(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".docz.yaml"),
		"api:\n  enabled: false\n  additional_docs:\n    - ../../etc/passwd\n")

	cfg, err := config.Load("", root)
	if err != nil {
		t.Fatalf("config.Load() = %v, want nil", err)
	}
	if _, err := cfg.Validate(); err != nil {
		t.Errorf("cfg.Validate() = %v, want nil while the block is dormant", err)
	}
}

// TestExternalConsumerDerivesTitle covers the other half of R10: the
// display title for a document that has no frontmatter to read one
// from, which is the whole reason additional_docs can be bare strings.
func TestExternalConsumerDerivesTitle(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "h1 with inline markup",
			content: "# **Contributing** to `docz`\n\nBody.\n",
			want:    "Contributing to docz",
		},
		{
			name:    "h1 after frontmatter",
			content: "---\nid: RFC-0001\n---\n\n# RFC-0001: A Title\n",
			want:    "RFC-0001: A Title",
		},
		{
			name:    "setext title",
			content: "Development\n===========\n\nBody.\n",
			want:    "Development",
		},
		{
			// "" is a normal outcome, not an error — the consumer falls
			// back to the filename, the way it already does for a
			// changelog with no version headings.
			name:    "no h1 at all",
			content: "Just prose, no heading.\n",
			want:    "",
		},
		{
			name:    "frontmatter only",
			content: "---\nid: RFC-0001\ntitle: \"In The Frontmatter\"\n---\n",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := docparse.Title([]byte(tt.content)); got != tt.want {
				t.Errorf("docparse.Title() = %q, want %q", got, tt.want)
			}
		})
	}
}
