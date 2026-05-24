// Package config provides configuration loading and merging for docz.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

// TypeConfig holds configuration for a single document type.
type TypeConfig struct {
	Enabled     bool     `mapstructure:"enabled"      yaml:"enabled"`
	Dir         string   `mapstructure:"dir"          yaml:"dir"`
	Template    string   `mapstructure:"template"     yaml:"template"`
	IDPrefix    string   `mapstructure:"id_prefix"    yaml:"id_prefix"`
	IDWidth     int      `mapstructure:"id_width"     yaml:"id_width"`
	Statuses    []string `mapstructure:"statuses"     yaml:"statuses"`
	StatusField string   `mapstructure:"status_field" yaml:"status_field"`
}

// IndexConfig holds configuration for index/README generation.
type IndexConfig struct {
	AutoUpdate     bool `mapstructure:"auto_update"     yaml:"auto_update"`
	PreserveHeader bool `mapstructure:"preserve_header" yaml:"preserve_header"`
}

// AuthorConfig holds configuration for author resolution.
type AuthorConfig struct {
	FromGit bool   `mapstructure:"from_git" yaml:"from_git"`
	Default string `mapstructure:"default"  yaml:"default"`
}

// WikiConfig holds configuration for the wiki/MkDocs integration.
type WikiConfig struct {
	AutoUpdate         bool              `mapstructure:"auto_update"         yaml:"auto_update"`
	MkDocsPath         string            `mapstructure:"mkdocs_path"         yaml:"mkdocs_path"`
	Plugins            []string          `mapstructure:"plugins"             yaml:"plugins,omitempty"`
	MarkdownExtensions []string          `mapstructure:"markdown_extensions" yaml:"markdown_extensions,omitempty"`
	Exclude            []string          `mapstructure:"exclude"             yaml:"exclude"`
	NavTitles          map[string]string `mapstructure:"nav_titles"          yaml:"nav_titles"`
	DocsDir            string            `mapstructure:"docs_dir"            yaml:"docs_dir,omitempty"`
	RepoURL            string            `mapstructure:"repo_url"            yaml:"repo_url,omitempty"`
	SiteURL            string            `mapstructure:"site_url"            yaml:"site_url,omitempty"`
	Theme              string            `mapstructure:"theme"               yaml:"theme,omitempty"`
}

// ToCConfig holds configuration for table of contents generation.
type ToCConfig struct {
	Enabled     bool `mapstructure:"enabled"      yaml:"enabled"`
	MinHeadings int  `mapstructure:"min_headings" yaml:"min_headings"`
}

// Config is the top-level configuration for docz.
type Config struct {
	DocsDir string                `mapstructure:"docs_dir" yaml:"docs_dir"`
	Types   map[string]TypeConfig `mapstructure:"types"    yaml:"types"`
	Index   IndexConfig           `mapstructure:"index"    yaml:"index"`
	Author  AuthorConfig          `mapstructure:"author"   yaml:"author"`
	Wiki    WikiConfig            `mapstructure:"wiki"     yaml:"wiki"`
	ToC     ToCConfig             `mapstructure:"toc"      yaml:"toc"`
}

// DefaultConfig returns the built-in default configuration.
func DefaultConfig() Config {
	return Config{
		DocsDir: "docs",
		Types: map[string]TypeConfig{
			"rfc": {
				Enabled:     true,
				Dir:         "rfc",
				IDPrefix:    "RFC",
				IDWidth:     4,
				Statuses:    []string{"Draft", "Proposed", "Accepted", "Rejected", "Superseded"},
				StatusField: "status",
			},
			"adr": {
				Enabled:     true,
				Dir:         "adr",
				IDPrefix:    "ADR",
				IDWidth:     4,
				Statuses:    []string{"Proposed", "Accepted", "Deprecated", "Superseded"},
				StatusField: "status",
			},
			"design": {
				Enabled:     true,
				Dir:         "design",
				IDPrefix:    "DESIGN",
				IDWidth:     4,
				Statuses:    []string{"Draft", "In Review", "Approved", "Implemented", "Abandoned"},
				StatusField: "status",
			},
			"impl": {
				Enabled:     true,
				Dir:         "impl",
				IDPrefix:    "IMPL",
				IDWidth:     4,
				Statuses:    []string{"Draft", "In Progress", "Completed", "Paused", "Cancelled"},
				StatusField: "status",
			},
			"plan": {
				Enabled:     true,
				Dir:         "plan",
				IDPrefix:    "PLAN",
				IDWidth:     4,
				Statuses:    []string{"Draft", "In Progress", "Completed", "Cancelled"},
				StatusField: "status",
			},
			"investigation": {
				Enabled:  true,
				Dir:      "investigation",
				IDPrefix: "INV",
				IDWidth:  4,
				Statuses: []string{
					"Open",
					"In Progress",
					"Concluded",
					"Inconclusive",
					"Abandoned",
				},
				StatusField: "status",
			},
		},
		Index: IndexConfig{
			AutoUpdate:     true,
			PreserveHeader: true,
		},
		Author: AuthorConfig{
			FromGit: true,
		},
		Wiki: WikiConfig{
			AutoUpdate: true,
			MkDocsPath: MkDocsFileName,
			Plugins:    []string{"techdocs-core"},
			Exclude:    []string{TemplatesDir, "examples"},
			NavTitles:  DefaultNavTitles(),
		},
		ToC: ToCConfig{
			Enabled:     true,
			MinHeadings: defaultMinHeadings,
		},
	}
}

// Load reads configuration from the global (~/.docz.yaml) and repo-root
// (.docz.yaml) config files, deep-merging them with repo root taking
// precedence. Built-in defaults are applied for any missing keys.
//
// If the repo-root .docz.yaml declares a top-level `types:` block, that
// list is treated as a REPLACEMENT of the default types map: only the
// types named there are kept on the returned Config. Omitting the
// `types:` block keeps all six built-in types. This is the INV-0003 fix
// implemented in IMPL-0006 Phase 5.
//
// If configFile is non-empty, it is used as the sole config source
// (no merge); the same types-replace-on-presence rule applies.
func Load(configFile string) (Config, error) {
	cfg := DefaultConfig()

	if configFile != "" {
		return loadFromFile(configFile, &cfg)
	}

	v := viper.New()

	// Load global config first.
	if home, err := os.UserHomeDir(); err == nil {
		if mergeErr := mergeConfigFile(v, filepath.Join(home, ConfigFileName)); mergeErr != nil {
			return cfg, mergeErr
		}
	}

	// Load repo-root config on top (deep merge, repo wins).
	if mergeErr := mergeConfigFile(v, ConfigFileName); mergeErr != nil {
		return cfg, mergeErr
	}

	if err := v.Unmarshal(&cfg); err != nil {
		return cfg, err
	}

	applyTypesReplaceOnPresence(&cfg, ConfigFileName)

	return cfg, nil
}

// TypeDir returns the full path to a type's directory relative to the repo
// root, e.g. "docs/rfc".
func (c *Config) TypeDir(docType string) string {
	tc, ok := c.Types[docType]
	if !ok {
		return filepath.Join(c.DocsDir, docType)
	}
	return filepath.Join(c.DocsDir, tc.Dir)
}

// DefaultNavTitles returns the default directory-to-nav-title mapping for
// docz-managed type directories.
func DefaultNavTitles() map[string]string {
	return map[string]string{
		"rfc":           "RFCs",
		"adr":           "ADRs",
		"design":        "Design",
		"impl":          "Implementation Plans",
		"plan":          "Plans",
		"investigation": "Investigations",
	}
}

// ValidTypes returns the list of built-in document type names.
func ValidTypes() []string {
	return []string{"rfc", "adr", "design", "impl", "plan", "investigation"}
}

// typeAliases maps short or alternate names to their canonical type name.
var typeAliases = map[string]string{
	"implementation": "impl",
	"inv":            "investigation",
}

// ResolveTypeAlias returns the canonical type name for the given input.
// If the input is already a canonical name or has no alias, it is returned as-is.
func ResolveTypeAlias(name string) string {
	if canonical, ok := typeAliases[name]; ok {
		return canonical
	}
	return name
}

// TypesHelp returns a formatted help string listing all valid types with aliases.
func TypesHelp() string {
	return `Document types:
  rfc              Request for Comments — high-level proposals
  adr              Architecture Decision Records — technical decisions
  design           Design documents — detailed feature designs
  impl             Implementation plans (alias: implementation)
  plan             Planning documents — goal, approach, components
  investigation    Research spikes — validate theories and errors (alias: inv)`
}

// Validate checks the configuration for common errors and returns a list of
// warnings and the first error found (if any).
func (c *Config) Validate() ([]string, error) {
	var warnings []string

	if c.DocsDir == "" {
		return warnings, errors.New("docs_dir must not be empty")
	}

	validTypes := map[string]bool{}
	for _, t := range ValidTypes() {
		validTypes[t] = true
	}

	for name, tc := range c.Types {
		if !validTypes[name] {
			warnings = append(warnings, fmt.Sprintf("unknown document type %q in config", name))
		}
		if tc.Enabled && len(tc.Statuses) == 0 {
			return warnings, fmt.Errorf("type %q has no statuses defined", name)
		}
	}

	return warnings, nil
}

// mergeConfigFile reads a YAML config file and merges it into v. A missing
// file is treated as "not configured" and silently skipped. Anything else
// (permission denied, malformed YAML, etc.) is surfaced as a wrapped error
// so the user sees a clear message instead of a silently half-defaulted
// config — see IMPL-0006 Phase 4.
func mergeConfigFile(v *viper.Viper, path string) error {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("checking config file %s: %w", path, err)
	}
	fileV := viper.New()
	fileV.SetConfigFile(path)
	if err := fileV.ReadInConfig(); err != nil {
		return fmt.Errorf("parsing config file %s: %w", path, err)
	}
	return v.MergeConfigMap(fileV.AllSettings())
}

func loadFromFile(path string, defaults *Config) (Config, error) {
	v := viper.New()
	v.SetConfigFile(path)

	if err := v.ReadInConfig(); err != nil {
		return *defaults, err
	}

	cfg := *defaults
	if err := v.Unmarshal(&cfg); err != nil {
		return *defaults, err
	}

	applyTypesReplaceOnPresence(&cfg, path)

	return cfg, nil
}

// applyTypesReplaceOnPresence enforces the INV-0003 contract: when the
// user's YAML at path declares a top-level `types:` map, only the named
// types are retained on cfg. Types listed by the user but not present in
// the built-in default set are dropped silently (unknown types are
// surfaced separately by Validate).
//
// If the file does not exist, cannot be parsed, or has no `types:` key,
// cfg is left untouched and the merge-based behavior continues.
func applyTypesReplaceOnPresence(cfg *Config, path string) {
	listed := userListedTypeNames(path)
	if listed == nil {
		return
	}

	filtered := make(map[string]TypeConfig, len(listed))
	for _, name := range listed {
		if tc, ok := cfg.Types[name]; ok {
			filtered[name] = tc
		}
	}
	cfg.Types = filtered
}

// userListedTypeNames returns the keys of the top-level `types:` map in
// the YAML file at path, or nil if the file is missing, malformed, or
// has no `types:` key. Parse errors from a malformed file are intentionally
// swallowed here because mergeConfigFile / loadFromFile already surface
// them via the main load path; this helper only decides the
// replace-vs-merge mode for the types map.
func userListedTypeNames(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil
	}
	typesNode, ok := raw["types"].(map[string]any)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(typesNode))
	for k := range typesNode {
		names = append(names, k)
	}
	return names
}
