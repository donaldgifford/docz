package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/pkg/doczcore/config"
)

// writeConfig writes a .docz.yaml into a fresh temp dir and returns the
// dir, so each case loads in isolation.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, config.ConfigFileName)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return dir
}

// TestLoad_ChangelogBlock covers the merge and normalization contract of
// the changelog: block (DESIGN-0010): omitted keys inherit defaults, an
// explicitly empty file backfills, and "./" prefixes are stripped.
func TestLoad_ChangelogBlock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		wantEnabled bool
		wantFile    string
	}{
		{
			name:        "absent block keeps defaults",
			content:     "docs_dir: docs\n",
			wantEnabled: false,
			wantFile:    config.DefaultChangelogFile,
		},
		{
			name:        "partial block keeps file default",
			content:     "changelog:\n  enabled: true\n",
			wantEnabled: true,
			wantFile:    config.DefaultChangelogFile,
		},
		{
			name:        "explicit empty file backfills default",
			content:     "changelog:\n  enabled: true\n  file: \"\"\n",
			wantEnabled: true,
			wantFile:    config.DefaultChangelogFile,
		},
		{
			name:        "whitespace-only file backfills default",
			content:     "changelog:\n  enabled: true\n  file: \"   \"\n",
			wantEnabled: true,
			wantFile:    config.DefaultChangelogFile,
		},
		{
			name:        "full block honored",
			content:     "changelog:\n  enabled: true\n  file: charts/foo/CHANGELOG.md\n",
			wantEnabled: true,
			wantFile:    "charts/foo/CHANGELOG.md",
		},
		{
			name:        "leading dot-slash normalized",
			content:     "changelog:\n  enabled: true\n  file: ./CHANGELOG.md\n",
			wantEnabled: true,
			wantFile:    config.DefaultChangelogFile,
		},
		{
			name:        "repeated dot-slash normalized",
			content:     "changelog:\n  enabled: true\n  file: ././docs/CHANGELOG.md\n",
			wantEnabled: true,
			wantFile:    "docs/CHANGELOG.md",
		},
		{
			name:        "file without enabled stays dormant",
			content:     "changelog:\n  file: docs/CHANGELOG.md\n",
			wantEnabled: false,
			wantFile:    "docs/CHANGELOG.md",
		},
		{
			name: "unknown sibling key tolerated",
			content: "changelog:\n  enabled: true\n  not_a_real_key: 42\n" +
				"  file: CHANGELOG.md\n",
			wantEnabled: true,
			wantFile:    config.DefaultChangelogFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := config.Load("", writeConfig(t, tt.content))
			if err != nil {
				t.Fatalf("Load() = %v, want nil", err)
			}
			if cfg.Changelog.Enabled != tt.wantEnabled {
				t.Errorf("Changelog.Enabled = %t, want %t",
					cfg.Changelog.Enabled, tt.wantEnabled)
			}
			if cfg.Changelog.File != tt.wantFile {
				t.Errorf("Changelog.File = %q, want %q",
					cfg.Changelog.File, tt.wantFile)
			}
		})
	}
}

// TestLoad_ChangelogBlockExplicitConfigFile pins that the --config path
// (loadFromFile) normalizes identically to the merged path.
func TestLoad_ChangelogBlockExplicitConfigFile(t *testing.T) {
	t.Parallel()
	dir := writeConfig(t, "changelog:\n  enabled: true\n  file: ./CHANGELOG.md\n")

	cfg, err := config.Load(filepath.Join(dir, config.ConfigFileName), "")
	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	if cfg.Changelog.File != config.DefaultChangelogFile {
		t.Errorf("Changelog.File = %q, want %q via the explicit-config path",
			cfg.Changelog.File, config.DefaultChangelogFile)
	}
}

// TestValidate_ChangelogFile covers Decision 5 (what a bad path is) and
// Decision 7 (validation applies only to an enabled block): every
// invalid value below must be accepted while the block is dormant.
func TestValidate_ChangelogFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		file    string
		wantErr string // substring; empty means valid
	}{
		{name: "default", file: config.DefaultChangelogFile},
		{name: "subpath", file: "charts/foo/CHANGELOG.md"},
		{name: "nested subpath", file: "a/b/c/CHANGELOG.md"},
		{name: "absolute", file: "/etc/CHANGELOG.md", wantErr: "must be relative"},
		{name: "parent traversal", file: "../CHANGELOG.md", wantErr: "must not traverse"},
		{name: "interior traversal", file: "charts/../../x.md", wantErr: "must not traverse"},
		{name: "trailing slash", file: "charts/", wantErr: "must be a file path"},
		{name: "empty", file: "", wantErr: "must not be empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			enabled := config.DefaultConfig()
			enabled.Changelog = config.ChangelogConfig{Enabled: true, File: tt.file}
			_, err := enabled.Validate()

			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("Validate() = %v, want nil for file %q", err, tt.file)
			case tt.wantErr != "" && err == nil:
				t.Errorf("Validate() = nil, want error containing %q for file %q",
					tt.wantErr, tt.file)
			case tt.wantErr != "" && err != nil && !strings.Contains(err.Error(), tt.wantErr):
				t.Errorf("Validate() = %v, want error containing %q", err, tt.wantErr)
			}

			// Decision 7: the identical value is fine while dormant.
			dormant := config.DefaultConfig()
			dormant.Changelog = config.ChangelogConfig{Enabled: false, File: tt.file}
			if _, err := dormant.Validate(); err != nil {
				t.Errorf("Validate() = %v, want nil: a disabled block must never fail load", err)
			}
		})
	}
}

// TestLoad_DormantChangelogBlockNowDecodes is the INV-0005 F2 rollout
// handshake: repos were told they could add the block before docz
// understood it (v1.0.0 ignored the unknown key). Those same files must
// now decode into the typed field rather than being dropped — and a
// dormant block with a path v1.1.0 would reject when enabled still must
// not fail load.
func TestLoad_DormantChangelogBlockNowDecodes(t *testing.T) {
	t.Parallel()
	dir := writeConfig(t, `docs_dir: docs
changelog:
  enabled: false
  file: ../shared/CHANGELOG.md
`)

	cfg, err := config.Load("", dir)
	if err != nil {
		t.Fatalf("Load() = %v, want nil: a dormant block must not fail load", err)
	}
	if cfg.Changelog.File != "../shared/CHANGELOG.md" {
		t.Errorf("Changelog.File = %q, want the block to decode verbatim", cfg.Changelog.File)
	}
	if _, err := cfg.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil while the block is dormant", err)
	}
}
