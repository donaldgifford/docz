package config_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/donaldgifford/docz/pkg/doczcore/config"
)

// TestLoad_APIBlockNormalization covers the decode and normalization half
// of the api: block contract (DESIGN-0011). Validation is judged
// separately, in TestValidate_API — Load never rejects.
func TestLoad_APIBlockNormalization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		wantEnabled bool
		wantLanding string
		wantExclude []string
		wantExtra   []string
	}{
		{
			name:        "absent block keeps defaults",
			content:     "docs_dir: docs\n",
			wantLanding: "",
		},
		{
			name:        "enabled alone backfills the landing page",
			content:     "api:\n  enabled: true\n",
			wantEnabled: true,
			wantLanding: "docs/index.md",
		},
		{
			// The backfill is under docs_dir, not a hardcoded "docs/", so
			// a repo that renamed the directory still resolves (Decision 3).
			name:        "backfill follows a non-default docs_dir",
			content:     "docs_dir: documentation\napi:\n  enabled: true\n",
			wantEnabled: true,
			wantLanding: "documentation/index.md",
		},
		{
			// A dormant block is left exactly as written, so `docz config`
			// never implies a landing page nothing will read.
			name:        "disabled block is not backfilled",
			content:     "api:\n  enabled: false\n",
			wantLanding: "",
		},
		{
			name:        "explicit landing page wins over the backfill",
			content:     "api:\n  enabled: true\n  landing_page: docs/home.md\n",
			wantEnabled: true,
			wantLanding: "docs/home.md",
		},
		{
			name: "dot-slash stripped from all three fields",
			content: "api:\n  enabled: true\n  landing_page: ././docs/index.md\n" +
				"  exclude:\n    - ./templates\n" +
				"  additional_docs:\n    - ././README.md\n",
			wantEnabled: true,
			wantLanding: "docs/index.md",
			wantExclude: []string{"templates"},
			wantExtra:   []string{"README.md"},
		},
		{
			// One stored spelling per exclude entry: a consumer matches the
			// deny-list by prefix, and "templates/" compared as
			// "templates//…" would match nothing and publish the files the
			// repo meant to withhold.
			name: "exclude trailing slashes collapse to one spelling",
			content: "api:\n  enabled: true\n" +
				"  exclude:\n    - templates/\n    - examples\n    - ./drafts/\n",
			wantEnabled: true,
			wantLanding: "docs/index.md",
			wantExclude: []string{"templates", "examples", "drafts"},
		},
		{
			// Normalizing is not sanitizing: ".." has to reach Validate
			// intact, or a traversal attempt would be silently laundered
			// into a path that validates clean.
			name: "traversal survives normalization for Validate to reject",
			content: "api:\n  enabled: true\n" +
				"  additional_docs:\n    - ../secrets.md\n",
			wantEnabled: true,
			wantLanding: "docs/index.md",
			wantExtra:   []string{"../secrets.md"},
		},
		{
			// "/" would normalize to "" if the trailing slash were stripped
			// unconditionally; kept whole so Validate can call it absolute
			// rather than empty.
			name: "bare slash exclude is preserved for the better message",
			content: "api:\n  enabled: true\n" +
				"  exclude:\n    - /\n",
			wantEnabled: true,
			wantLanding: "docs/index.md",
			wantExclude: []string{"/"},
		},
		{
			name: "unknown sibling key tolerated",
			content: "api:\n  enabled: true\n  not_a_real_key: 42\n" +
				"  landing_page: docs/index.md\n",
			wantEnabled: true,
			wantLanding: "docs/index.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := config.Load("", writeConfig(t, tt.content))
			if err != nil {
				t.Fatalf("Load() = %v, want nil", err)
			}
			if cfg.API.Enabled != tt.wantEnabled {
				t.Errorf("API.Enabled = %t, want %t", cfg.API.Enabled, tt.wantEnabled)
			}
			if cfg.API.LandingPage != tt.wantLanding {
				t.Errorf("API.LandingPage = %q, want %q",
					cfg.API.LandingPage, tt.wantLanding)
			}
			if !slices.Equal(cfg.API.Exclude, tt.wantExclude) {
				t.Errorf("API.Exclude = %q, want %q", cfg.API.Exclude, tt.wantExclude)
			}
			if !slices.Equal(cfg.API.AdditionalDocs, tt.wantExtra) {
				t.Errorf("API.AdditionalDocs = %q, want %q",
					cfg.API.AdditionalDocs, tt.wantExtra)
			}
		})
	}
}

// TestLoad_APIBlockExplicitConfigFile pins that the --config path
// (loadFromFile) normalizes identically to the merged path. The two have
// separate call sites, so a fix applied to one can silently miss the other.
func TestLoad_APIBlockExplicitConfigFile(t *testing.T) {
	t.Parallel()

	dir := writeConfig(t, "api:\n  enabled: true\n  exclude:\n    - ./templates/\n")
	cfg, err := config.Load(filepath.Join(dir, config.ConfigFileName), "")
	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	if cfg.API.LandingPage != "docs/index.md" {
		t.Errorf("API.LandingPage = %q, want %q via the explicit-config path",
			cfg.API.LandingPage, "docs/index.md")
	}
	if want := []string{"templates"}; !slices.Equal(cfg.API.Exclude, want) {
		t.Errorf("API.Exclude = %q, want %q via the explicit-config path",
			cfg.API.Exclude, want)
	}
}
