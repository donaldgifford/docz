package consumer

// The changelog surface proof (IMPL-0015 Phase 3): the config block and
// ParseChangelog exercised from outside the module, in the shape
// docz-api will freeze as contract clause R6.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/donaldgifford/docz/pkg/doczcore/config"
	"github.com/donaldgifford/docz/pkg/doczcore/document"
)

const changelogYAML = `docs_dir: docs
changelog:
  enabled: true
  file: ./charts/api/CHANGELOG.md
`

// fleetChangelog is the git-cliff shape every fleet repo emits:
// preamble, an unreleased section, then dated releases with scoped
// commit bullets and PR links.
const fleetChangelog = `# Changelog

All notable changes to this project are documented here.

## [unreleased]

### Bug Fixes

- *(chart)* Scope the main Service selector ([#12](https://example.com/12))

## [0.4.2] - 2026-07-23

### Bug Fixes

- *(ci)* Drop stale GPG signing ([#10](https://example.com/10))

### Miscellaneous Tasks

- *(release)* Cut v0.4.2
`

// TestExternalConsumerResolvesChangelogConfig covers the config half of
// the R6 contract: yaml keys, defaults, the enabled flag, and the
// load-time normalization consumers rely on when fetching the path out
// of a git tree.
func TestExternalConsumerResolvesChangelogConfig(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".docz.yaml"), changelogYAML)

	cfg, err := config.Load("", root)
	if err != nil {
		t.Fatalf("config.Load(\"\", %q) = %v, want nil", root, err)
	}
	if _, err := cfg.Validate(); err != nil {
		t.Fatalf("cfg.Validate() = %v, want nil", err)
	}

	if !cfg.Changelog.Enabled {
		t.Error("Changelog.Enabled = false, want true")
	}
	if want := "charts/api/CHANGELOG.md"; cfg.Changelog.File != want {
		t.Errorf("Changelog.File = %q, want %q (leading ./ normalized away)",
			cfg.Changelog.File, want)
	}

	// A repo with no changelog block still resolves the documented
	// defaults, so a consumer can read the field unconditionally.
	bare := t.TempDir()
	writeFile(t, filepath.Join(bare, ".docz.yaml"), "docs_dir: docs\n")
	bareCfg, err := config.Load("", bare)
	if err != nil {
		t.Fatalf("config.Load on a block-less repo = %v, want nil", err)
	}
	if bareCfg.Changelog.Enabled {
		t.Error("default Changelog.Enabled = true, want false")
	}
	if bareCfg.Changelog.File != config.DefaultChangelogFile {
		t.Errorf("default Changelog.File = %q, want %q",
			bareCfg.Changelog.File, config.DefaultChangelogFile)
	}
}

// TestExternalConsumerParsesChangelog covers the parser half: the
// version order and identity, group titles, item counts, preamble
// capture, and the ErrNoVersions sentinel docz-api pins.
func TestExternalConsumerParsesChangelog(t *testing.T) {
	cl, err := document.ParseChangelog([]byte(fleetChangelog))
	if err != nil {
		t.Fatalf("ParseChangelog() = %v, want nil", err)
	}

	if len(cl.Versions) != 2 {
		t.Fatalf("got %d versions, want 2", len(cl.Versions))
	}

	unreleased := cl.Versions[0]
	if !unreleased.Unreleased || unreleased.Version != "unreleased" {
		t.Errorf("versions[0] = %+v, want the unreleased section", unreleased)
	}
	if unreleased.Date != "" {
		t.Errorf("unreleased Date = %q, want empty", unreleased.Date)
	}

	released := cl.Versions[1]
	if released.Version != "0.4.2" {
		t.Errorf("versions[1].Version = %q, want %q", released.Version, "0.4.2")
	}
	if released.Date != "2026-07-23" {
		t.Errorf("versions[1].Date = %q, want %q", released.Date, "2026-07-23")
	}
	if len(released.Groups) != 2 {
		t.Fatalf("versions[1] has %d groups, want 2", len(released.Groups))
	}
	if released.Groups[0].Title != "Bug Fixes" {
		t.Errorf("groups[0].Title = %q, want %q", released.Groups[0].Title, "Bug Fixes")
	}
	if len(released.Groups[0].Items) != 1 {
		t.Errorf("groups[0] has %d items, want 1", len(released.Groups[0].Items))
	}
	// Item text keeps the git-cliff scope marker and PR link verbatim —
	// consumers render it as markdown.
	if got := released.Groups[0].Items[0]; got != "*(ci)* Drop stale GPG signing ([#10](https://example.com/10))" {
		t.Errorf("groups[0].Items[0] = %q, want the raw bullet body", got)
	}

	if cl.Preamble == "" {
		t.Error("Preamble is empty, want the title and intro prose")
	}

	// The sentinel contract: a file with no version headings is not a
	// changelog, and the result is nil.
	got, err := document.ParseChangelog([]byte("# Changelog\n\nNo releases yet.\n"))
	if !errors.Is(err, document.ErrNoVersions) {
		t.Errorf("ParseChangelog(no versions) = %v, want ErrNoVersions", err)
	}
	if got != nil {
		t.Errorf("ParseChangelog(no versions) = %+v, want nil alongside the error", got)
	}
}

// TestExternalConsumerReadsChangelogFromDisk is the end-to-end shape a
// consumer runs: resolve the configured path, read those bytes, parse.
func TestExternalConsumerReadsChangelogFromDisk(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".docz.yaml"),
		"changelog:\n  enabled: true\n  file: CHANGELOG.md\n")
	writeFile(t, filepath.Join(root, "CHANGELOG.md"), fleetChangelog)

	cfg, err := config.Load("", root)
	if err != nil {
		t.Fatalf("config.Load() = %v, want nil", err)
	}
	if !cfg.Changelog.Enabled {
		t.Fatal("changelog block not enabled")
	}

	raw, err := os.ReadFile(filepath.Join(root, cfg.Changelog.File))
	if err != nil {
		t.Fatalf("reading the configured changelog: %v", err)
	}

	cl, err := document.ParseChangelog(raw)
	if err != nil {
		t.Fatalf("ParseChangelog() = %v, want nil", err)
	}
	if len(cl.Versions) != 2 {
		t.Errorf("got %d versions, want 2", len(cl.Versions))
	}
}
