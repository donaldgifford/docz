package doczcontract_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	doczcfg "github.com/donaldgifford/docz/pkg/doczcore/config"
	doczdoc "github.com/donaldgifford/docz/pkg/doczcore/document"
)

// R6 — the docz v1.1.0 changelog surface (upstream DESIGN-0010): the
// changelog: config block's defaults, merge, and normalization, and
// ParseChangelog's parse shape and sentinel. Feature 2 (per-document
// backlinks) will join on the bare-semver version identity pinned here,
// so a drift in any of these is a breaking change for docz-api even when
// the Go API still compiles.

// changelogFixture is a frozen fleet-shaped git-cliff output. It is a
// static snapshot (deliberately NOT the live repo CHANGELOG.md, which
// drifts): preamble, an unreleased section, and released versions
// including a v-prefixed heading to pin the v-trim normalization.
const changelogFixture = "testdata/changelog_fleet.md"

// loadChangelogConfig loads a .docz.yaml with the given body hermetically
// (same HOME-override pattern as loadConfig; a fresh temp repo dir per
// call so each case controls the full config).
func loadChangelogConfig(t *testing.T, body string) doczcfg.Config {
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

func TestChangelogConfigDefaults(t *testing.T) {
	if doczcfg.DefaultChangelogFile != "CHANGELOG.md" {
		t.Errorf("DefaultChangelogFile = %q, want %q", doczcfg.DefaultChangelogFile, "CHANGELOG.md")
	}

	cl := doczcfg.DefaultConfig().Changelog
	if cl.Enabled {
		t.Error("DefaultConfig().Changelog.Enabled = true, want false (opt-in)")
	}
	if cl.File != doczcfg.DefaultChangelogFile {
		t.Errorf("DefaultConfig().Changelog.File = %q, want %q", cl.File, doczcfg.DefaultChangelogFile)
	}
}

func TestChangelogConfigLoad(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		wantEnabled bool
		wantFile    string
	}{
		{"absent block keeps defaults", "docs_dir: docs\n", false, "CHANGELOG.md"},
		{"partial block keeps file default", "changelog:\n  enabled: true\n", true, "CHANGELOG.md"},
		{"explicit empty file backfills default", "changelog:\n  enabled: true\n  file: \"\"\n", true, "CHANGELOG.md"},
		{"leading dot-slash normalized", "changelog:\n  enabled: true\n  file: ./CHANGELOG.md\n", true, "CHANGELOG.md"},
		{"subpath kept for chart changelogs", "changelog:\n  enabled: true\n  file: charts/foo/CHANGELOG.md\n", true, "charts/foo/CHANGELOG.md"},
		{"file without enabled stays dormant", "changelog:\n  file: charts/foo/CHANGELOG.md\n", false, "charts/foo/CHANGELOG.md"},
		// The INV-0005 F2 rollout guarantee: unknown top-level keys never
		// break Load, so repos can adopt future config before the pin does.
		{"unknown sibling key tolerated", "changelog:\n  enabled: true\nfuture_key:\n  x: 1\n", true, "CHANGELOG.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := loadChangelogConfig(t, tt.yaml)
			if cfg.Changelog.Enabled != tt.wantEnabled {
				t.Errorf("Enabled = %v, want %v", cfg.Changelog.Enabled, tt.wantEnabled)
			}
			if cfg.Changelog.File != tt.wantFile {
				t.Errorf("File = %q, want %q", cfg.Changelog.File, tt.wantFile)
			}
		})
	}
}

func TestChangelogConfigValidateRejectsBadPathWhenEnabled(t *testing.T) {
	cfg := loadChangelogConfig(t, "changelog:\n  enabled: true\n  file: ../escape.md\n")
	if _, err := cfg.Validate(); err == nil {
		t.Error("Validate() = nil for enabled traversal path, want error")
	}

	// Disabled blocks are dormant: the same bad path must NOT fail
	// validation, or un-opted-in repos would break on future defaults.
	cfg = loadChangelogConfig(t, "changelog:\n  enabled: false\n  file: ../escape.md\n")
	if _, err := cfg.Validate(); err != nil {
		t.Errorf("Validate() = %v for disabled block, want nil (unvalidated when dormant)", err)
	}
}

func TestParseChangelogFleetFixture(t *testing.T) {
	raw, err := os.ReadFile(changelogFixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	cl, err := doczdoc.ParseChangelog(raw)
	if err != nil {
		t.Fatalf("ParseChangelog: %v", err)
	}

	if cl.Preamble == "" {
		t.Error("Preamble is empty, want the verbatim pre-heading bytes")
	}

	if len(cl.Versions) != 3 {
		t.Fatalf("len(Versions) = %d, want 3", len(cl.Versions))
	}

	unrel := cl.Versions[0]
	if unrel.Version != "unreleased" || !unrel.Unreleased || unrel.Date != "" {
		t.Errorf("Versions[0] = {%q %v %q}, want {\"unreleased\" true \"\"}",
			unrel.Version, unrel.Unreleased, unrel.Date)
	}
	if len(unrel.Groups) != 2 {
		t.Errorf("unreleased groups = %d, want 2", len(unrel.Groups))
	}

	rel := cl.Versions[1]
	if rel.Version != "0.4.2" || rel.Unreleased || rel.Date != "2026-07-23" {
		t.Errorf("Versions[1] = {%q %v %q}, want {\"0.4.2\" false \"2026-07-23\"}",
			rel.Version, rel.Unreleased, rel.Date)
	}
	if len(rel.Groups) != 1 || rel.Groups[0].Title != "Bug Fixes" {
		t.Fatalf("Versions[1].Groups = %+v, want one \"Bug Fixes\" group", rel.Groups)
	}
	// Items keep the raw markdown: scope marker and PR link intact, bullet
	// marker stripped.
	item := rel.Groups[0].Items[0]
	if want := "*(ci)* Drop stale signing of archives ([#10](https://github.com/example/repo/issues/10))"; item != want {
		t.Errorf("item = %q, want %q", item, want)
	}

	// The v-prefixed heading normalizes to bare semver — the version
	// identity feature 2 will join on.
	vtrim := cl.Versions[2]
	if vtrim.Version != "0.4.1" || vtrim.Date != "2026-07-22" {
		t.Errorf("Versions[2] = {%q %q}, want {\"0.4.1\" \"2026-07-22\"} (v-trim)",
			vtrim.Version, vtrim.Date)
	}
	if len(vtrim.Groups) != 1 || len(vtrim.Groups[0].Items) != 2 {
		t.Errorf("Versions[2].Groups = %+v, want one group with 2 items", vtrim.Groups)
	}
}

func TestParseChangelogNoVersions(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input string
	}{
		{"prose only", "# just prose\n\nno versions here\n"},
		{"empty input", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := doczdoc.ParseChangelog([]byte(tt.input)); !errors.Is(err, doczdoc.ErrNoVersions) {
				t.Errorf("ParseChangelog error = %v, want errors.Is ErrNoVersions", err)
			}
		})
	}
}
