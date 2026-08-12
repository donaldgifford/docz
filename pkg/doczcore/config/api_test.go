// Copyright 2026 Donald Gifford
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config_test

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
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

// TestValidate_API covers every rejection class of the api block, and —
// in the same subtest — pins the dormancy guarantee by asserting the
// identical value loads clean while the block is disabled (DESIGN-0011,
// the rule DESIGN-0010 Decision 7 established for changelog).
func TestValidate_API(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		api     config.APIConfig
		wantErr string // substring; empty means valid
	}{
		{
			name: "landing page and lists all valid",
			api: config.APIConfig{
				LandingPage:    "docs/index.md",
				Exclude:        []string{"templates", "examples/scratch"},
				AdditionalDocs: []string{"README.md", "CONTRIBUTING.md"},
			},
		},

		// landing_page names a file, so a trailing "/" is a mistake.
		{
			name:    "landing page absolute",
			api:     config.APIConfig{LandingPage: "/etc/index.md"},
			wantErr: "api.landing_page",
		},
		{
			name:    "landing page traversal",
			api:     config.APIConfig{LandingPage: "../index.md"},
			wantErr: "must not traverse",
		},
		{
			// Unreachable through Load, which backfills it, but reachable
			// for a hand-built Config.
			name:    "landing page empty",
			api:     config.APIConfig{LandingPage: ""},
			wantErr: "api.landing_page must not be empty",
		},
		{
			name:    "landing page is a directory",
			api:     config.APIConfig{LandingPage: "docs/"},
			wantErr: "must be a file path",
		},

		// exclude entries name directory prefixes: a trailing "/" is fine,
		// everything else is judged exactly as a file path is.
		{
			name: "exclude trailing slash allowed",
			api: config.APIConfig{
				LandingPage: "docs/index.md",
				Exclude:     []string{"templates/"},
			},
		},
		{
			name: "exclude traversal",
			api: config.APIConfig{
				LandingPage: "docs/index.md",
				Exclude:     []string{"ok", "../secrets"},
			},
			wantErr: `api.exclude[1] "../secrets" must not traverse`,
		},
		{
			name: "exclude empty entry",
			api: config.APIConfig{
				LandingPage: "docs/index.md",
				Exclude:     []string{""},
			},
			wantErr: "api.exclude[0] must not be empty",
		},
		{
			name: "exclude absolute",
			api: config.APIConfig{
				LandingPage: "docs/index.md",
				Exclude:     []string{"/etc"},
			},
			wantErr: "must be relative",
		},

		// additional_docs entries are repo-relative files.
		{
			name: "additional docs traversal",
			api: config.APIConfig{
				LandingPage:    "docs/index.md",
				AdditionalDocs: []string{"../../etc/passwd"},
			},
			wantErr: `api.additional_docs[0] "../../etc/passwd" must not traverse`,
		},
		{
			name: "additional docs empty entry",
			api: config.APIConfig{
				LandingPage:    "docs/index.md",
				AdditionalDocs: []string{"README.md", ""},
			},
			wantErr: "api.additional_docs[1] must not be empty",
		},
		{
			name: "additional docs duplicate",
			api: config.APIConfig{
				LandingPage:    "docs/index.md",
				AdditionalDocs: []string{"README.md", "CONTRIBUTING.md", "README.md"},
			},
			wantErr: "duplicates api.additional_docs[0]",
		},
		{
			name: "additional docs under docs_dir",
			api: config.APIConfig{
				LandingPage:    "docs/index.md",
				AdditionalDocs: []string{"docs/examples/one.md"},
			},
			wantErr: "already under docs_dir",
		},
		{
			name: "additional docs is docs_dir itself",
			api: config.APIConfig{
				LandingPage:    "docs/index.md",
				AdditionalDocs: []string{"docs"},
			},
			wantErr: "already under docs_dir",
		},
		{
			// A sibling directory that merely shares the prefix is fine —
			// "docsite" is not under "docs".
			name: "additional docs sharing a docs_dir prefix allowed",
			api: config.APIConfig{
				LandingPage:    "docs/index.md",
				AdditionalDocs: []string{"docsite/one.md"},
			},
		},

		// The route ambiguity that dropping the /docs/ namespace segment
		// leaves open: /:owner/:repo/design/notes.md is indistinguishable
		// from /:owner/:repo/:type/:docId.
		{
			name: "additional docs first segment is an enabled type",
			api: config.APIConfig{
				LandingPage:    "docs/index.md",
				AdditionalDocs: []string{"design/notes.md"},
			},
			wantErr: `reserved by the enabled "design" type`,
		},
		{
			// A consumer resolves a route segment case-insensitively, the
			// way resolveType does, so the collision check must too.
			name: "additional docs first segment collides case-insensitively",
			api: config.APIConfig{
				LandingPage:    "docs/index.md",
				AdditionalDocs: []string{"RFC/notes.md"},
			},
			wantErr: `reserved by the enabled "rfc" type`,
		},
		{
			// A route segment resolves through resolveType's full tier
			// list, so a registry alias shadows a type route exactly as the
			// canonical name does: /:owner/:repo/inv/… is the investigation
			// type. Found by security review — the first cut of this check
			// claimed only names and dirs.
			name: "additional docs first segment is a registry alias",
			api: config.APIConfig{
				LandingPage:    "docs/index.md",
				AdditionalDocs: []string{"inv/notes.md"},
			},
			wantErr: `reserved by the enabled "investigation" type`,
		},
		{
			name: "additional docs first segment is a long registry alias",
			api: config.APIConfig{
				LandingPage:    "docs/index.md",
				AdditionalDocs: []string{"implementation/x.md"},
			},
			wantErr: `reserved by the enabled "impl" type`,
		},
		{
			name: "additional docs first segment is an id_prefix",
			api: config.APIConfig{
				LandingPage:    "docs/index.md",
				AdditionalDocs: []string{"DESIGN/notes.md"},
			},
			wantErr: `reserved by the enabled "design" type`,
		},
		{
			// Git stores a file literally named "rfc%2Fnotes.md", and a
			// router that percent-decodes before matching sees the
			// two-segment form. The raw path alone is not the route.
			name: "additional docs percent-encoded separator",
			api: config.APIConfig{
				LandingPage:    "docs/index.md",
				AdditionalDocs: []string{"rfc%2Fnotes.md"},
			},
			wantErr: `reserved by the enabled "rfc" type`,
		},
		{
			// plan ships disabled, so it claims no route segment and the
			// entry is unambiguous. The check tracks the enabled set, not
			// the built-in catalog.
			name: "additional docs first segment is a disabled type",
			api: config.APIConfig{
				LandingPage:    "docs/index.md",
				AdditionalDocs: []string{"plan/notes.md"},
			},
		},

		// Two spellings of one file are one file to any consumer that
		// folds case, and publishing it twice is what the duplicate rule
		// exists to stop.
		{
			name: "additional docs duplicate differing only in case",
			api: config.APIConfig{
				LandingPage:    "docs/index.md",
				AdditionalDocs: []string{"README.md", "readme.md"},
			},
			wantErr: "duplicates api.additional_docs[0]",
		},
		{
			name: "additional docs also names the landing page",
			api: config.APIConfig{
				LandingPage:    "CONTRIBUTING.md",
				AdditionalDocs: []string{"CONTRIBUTING.md"},
			},
			wantErr: "also names api.landing_page",
		},
		{
			name: "additional docs under docs_dir differing only in case",
			api: config.APIConfig{
				LandingPage:    "docs/index.md",
				AdditionalDocs: []string{"DOCS/secret.md"},
			},
			wantErr: "already under docs_dir",
		},

		// A landing page the repo has also told docz never to publish is a
		// config that contradicts itself: the consumer withholds the file
		// and the front page 404s.
		{
			name: "landing page is excluded",
			api: config.APIConfig{
				LandingPage: "docs/private/home.md",
				Exclude:     []string{"private"},
			},
			wantErr: `excluded by api.exclude[0] "private"`,
		},
		{
			name: "landing page is excluded by a trailing-slash entry",
			api: config.APIConfig{
				LandingPage: "docs/private/home.md",
				Exclude:     []string{"private/"},
			},
			wantErr: "excluded by api.exclude[0]",
		},
		{
			// templates holds docz's own override machinery and is excluded
			// unconditionally, so no exclude entry names it.
			name: "landing page is under templates",
			api: config.APIConfig{
				LandingPage: "docs/templates/rfc.md",
			},
			wantErr: `is under "docs/templates", which is never published`,
		},
		{
			// The prefix has to be a whole path segment: "privateer" is not
			// inside "private".
			name: "landing page merely sharing an exclude prefix allowed",
			api: config.APIConfig{
				LandingPage: "docs/privateer/home.md",
				Exclude:     []string{"private"},
			},
		},
		{
			// Only the first segment addresses a route; deeper ones are
			// ordinary directory names.
			name: "type name deeper in the path allowed",
			api: config.APIConfig{
				LandingPage:    "docs/index.md",
				AdditionalDocs: []string{"internal/design/notes.md"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			enabled := config.DefaultConfig()
			enabled.API = tt.api
			enabled.API.Enabled = true
			_, err := enabled.Validate()

			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("Validate() = %v, want nil", err)
			case tt.wantErr != "" && err == nil:
				t.Errorf("Validate() = nil, want error containing %q", tt.wantErr)
			case tt.wantErr != "" && err != nil && !strings.Contains(err.Error(), tt.wantErr):
				t.Errorf("Validate() = %v, want error containing %q", err, tt.wantErr)
			}

			// Every rejection wraps the sentinel, so a consumer can tell
			// "this repo's api paths are bad" from any other config failure
			// without matching on the message.
			if tt.wantErr != "" && err != nil && !errors.Is(err, config.ErrInvalidAPIPath) {
				t.Errorf("Validate() = %v, want it to wrap ErrInvalidAPIPath", err)
			}

			// The dormancy guarantee, asserted against the same value: a
			// disabled block must never fail load.
			dormant := config.DefaultConfig()
			dormant.API = tt.api
			dormant.API.Enabled = false
			if _, err := dormant.Validate(); err != nil {
				t.Errorf("Validate() = %v, want nil: a disabled block must never fail load", err)
			}
		})
	}
}

// TestValidate_APIFollowsNonDefaultDocsDir pins that the docs_dir overlap
// rule reads the configured directory rather than a hardcoded "docs": a
// repo that renamed it must get the same protection, and must not be
// rejected for a path that only collides with the default name.
func TestValidate_APIFollowsNonDefaultDocsDir(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.DocsDir = "documentation"
	cfg.API = config.APIConfig{
		Enabled:        true,
		LandingPage:    "documentation/index.md",
		AdditionalDocs: []string{"docs/legacy.md"},
	}
	if _, err := cfg.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil: %q is not under docs_dir %q",
			err, "docs/legacy.md", cfg.DocsDir)
	}

	cfg.API.AdditionalDocs = []string{"documentation/legacy.md"}
	_, err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "already under docs_dir") {
		t.Errorf("Validate() = %v, want the docs_dir overlap rejection", err)
	}
}

// TestValidate_APIUncleanDocsDir pins that the overlap rule survives a
// docs_dir spelling that is not itself canonical. docs_dir is not run
// through the path validator, so the rule cannot assume it is clean: every
// value below resolves to "docs" on a consumer, and an entry inside it
// must be rejected for all of them.
func TestValidate_APIUncleanDocsDir(t *testing.T) {
	t.Parallel()

	for _, docsDir := range []string{"docs", "docs/", "docs/.", "docs//", "./docs", "docs/sub/.."} {
		t.Run(docsDir, func(t *testing.T) {
			t.Parallel()

			cfg := config.DefaultConfig()
			cfg.DocsDir = docsDir
			cfg.API = config.APIConfig{
				Enabled:        true,
				LandingPage:    "index.md",
				AdditionalDocs: []string{"docs/secret.md"},
			}
			_, err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), "already under docs_dir") {
				t.Errorf("Validate() = %v, want the docs_dir overlap rejection for docs_dir %q",
					err, docsDir)
			}
		})
	}
}

// TestValidate_APIRepoRootDocsDir pins the degenerate case: docs_dir "."
// means the whole repo is consumed automatically, so there is nothing
// left for additional_docs to add and every entry overlaps.
func TestValidate_APIRepoRootDocsDir(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.DocsDir = "."
	cfg.API = config.APIConfig{
		Enabled:        true,
		LandingPage:    "index.md",
		AdditionalDocs: []string{"CONTRIBUTING.md"},
	}
	_, err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "already under docs_dir") {
		t.Errorf("Validate() = %v, want the docs_dir overlap rejection", err)
	}
}

// TestValidate_APICustomTypeTokens pins that the reserved-segment set
// tracks the config's types rather than the built-in catalog: a custom
// type claims its name, its declared aliases, and its id_prefix, and a
// disabled one claims nothing.
func TestValidate_APICustomTypeTokens(t *testing.T) {
	t.Parallel()

	base := func() config.Config {
		cfg := config.DefaultConfig()
		cfg.Types["frameworks"] = config.TypeConfig{
			Enabled:  true,
			Dir:      "fw",
			IDPrefix: "FRM",
			IDWidth:  4,
			Statuses: []string{"Draft"},
			Aliases:  []string{"fmk"},
		}
		return cfg
	}

	for _, segment := range []string{"frameworks", "fw", "fmk", "FRM"} {
		t.Run("claimed/"+segment, func(t *testing.T) {
			t.Parallel()

			cfg := base()
			cfg.API = config.APIConfig{
				Enabled:        true,
				LandingPage:    "docs/index.md",
				AdditionalDocs: []string{segment + "/notes.md"},
			}
			_, err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), `reserved by the enabled "frameworks" type`) {
				t.Errorf("Validate() = %v, want %q rejected as a reserved route segment",
					err, segment)
			}
		})
	}

	t.Run("released when disabled", func(t *testing.T) {
		t.Parallel()

		cfg := base()
		tc := cfg.Types["frameworks"]
		tc.Enabled = false
		cfg.Types["frameworks"] = tc
		cfg.API = config.APIConfig{
			Enabled:        true,
			LandingPage:    "docs/index.md",
			AdditionalDocs: []string{"fw/notes.md"},
		}
		if _, err := cfg.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil: a disabled type claims no route segment", err)
		}
	})
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

// TestLoad_DormantAPIBlockNowDecodes is the rollout handshake, the same
// one IMPL-0015 added for changelog: repos were told they could commit the
// block before docz understood it, and v1.1.x ignored the unknown key.
// Those same files must now decode into the typed field rather than being
// dropped — and a dormant block holding paths this version would reject
// when enabled still must not fail load.
func TestLoad_DormantAPIBlockNowDecodes(t *testing.T) {
	t.Parallel()

	dir := writeConfig(t, `docs_dir: docs
api:
  enabled: false
  landing_page: ../elsewhere/index.md
  exclude:
    - ../secrets
  additional_docs:
    - design/notes.md
    - docs/inside.md
`)

	cfg, err := config.Load("", dir)
	if err != nil {
		t.Fatalf("Load() = %v, want nil: a dormant block must not fail load", err)
	}
	if want := "../elsewhere/index.md"; cfg.API.LandingPage != want {
		t.Errorf("API.LandingPage = %q, want %q decoded verbatim", cfg.API.LandingPage, want)
	}
	if want := []string{"../secrets"}; !slices.Equal(cfg.API.Exclude, want) {
		t.Errorf("API.Exclude = %q, want %q decoded verbatim", cfg.API.Exclude, want)
	}
	want := []string{"design/notes.md", "docs/inside.md"}
	if !slices.Equal(cfg.API.AdditionalDocs, want) {
		t.Errorf("API.AdditionalDocs = %q, want %q decoded verbatim",
			cfg.API.AdditionalDocs, want)
	}
	if _, err := cfg.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil while the block is dormant", err)
	}
}
