// Package config provides configuration loading and merging for docz.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

// TypeConfig holds configuration for a single document type.
//
// PluralLabel is the human-readable section heading and (per Decisions §4
// of IMPL-0006) the single source for the README index "All ADRs", "All
// RFCs", "All Implementation Plans", etc. It also wins over a missing
// `WikiConfig.NavTitles` entry when rendering the wiki landing page.
// `WikiConfig.NavTitles[name]`, when set, still overrides `PluralLabel`
// for the wiki nav for one release; deprecation/removal of NavTitles is
// deferred to a future release.
type TypeConfig struct {
	Enabled     bool     `mapstructure:"enabled"      yaml:"enabled"`
	Dir         string   `mapstructure:"dir"          yaml:"dir"`
	Template    string   `mapstructure:"template"     yaml:"template"`
	IDPrefix    string   `mapstructure:"id_prefix"    yaml:"id_prefix"`
	IDWidth     int      `mapstructure:"id_width"     yaml:"id_width"`
	Statuses    []string `mapstructure:"statuses"     yaml:"statuses"`
	StatusField string   `mapstructure:"status_field" yaml:"status_field"`
	PluralLabel string   `mapstructure:"plural_label" yaml:"plural_label,omitempty"`
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
				PluralLabel: "RFCs",
			},
			"adr": {
				Enabled:     true,
				Dir:         "adr",
				IDPrefix:    "ADR",
				IDWidth:     4,
				Statuses:    []string{"Proposed", "Accepted", "Deprecated", "Superseded"},
				StatusField: "status",
				PluralLabel: "ADRs",
			},
			"design": {
				Enabled:     true,
				Dir:         "design",
				IDPrefix:    "DESIGN",
				IDWidth:     4,
				Statuses:    []string{"Draft", "In Review", "Approved", "Implemented", "Abandoned"},
				StatusField: "status",
				PluralLabel: "Design",
			},
			"impl": {
				Enabled:     true,
				Dir:         "impl",
				IDPrefix:    "IMPL",
				IDWidth:     4,
				Statuses:    []string{"Draft", "In Progress", "Completed", "Paused", "Cancelled"},
				StatusField: "status",
				PluralLabel: "Implementation Plans",
			},
			"plan": {
				Enabled:     true,
				Dir:         "plan",
				IDPrefix:    "PLAN",
				IDWidth:     4,
				Statuses:    []string{"Draft", "In Progress", "Completed", "Cancelled"},
				StatusField: "status",
				PluralLabel: "Plans",
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
				PluralLabel: "Investigations",
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
	fillTypeFieldDefaults(&cfg)

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

// ErrUnknownType is the sentinel returned by ValidateType when the input
// does not name a built-in document type. Callers can branch on it with
// errors.Is to render a custom hint without parsing the wrapped message.
var ErrUnknownType = errors.New("unknown document type")

// ValidateType canonicalizes and validates a user-supplied type name.
// It lowercases the input, resolves aliases (e.g. "inv" -> "investigation"),
// and verifies the result is in the configured Config.Types map. On
// success it returns the canonical name; on failure it returns a
// fmt.Errorf-wrapped ErrUnknownType.
//
// Callers that need the canonical name and want a single error site
// should use this helper instead of duplicating the lookup-and-format
// block at each CLI subcommand boundary (IMPL-0006 Phase 7).
func (c *Config) ValidateType(name string) (string, error) {
	canonical := ResolveTypeAlias(strings.ToLower(name))
	if _, ok := c.Types[canonical]; !ok {
		return "", fmt.Errorf("%w %q (valid types: %s)",
			ErrUnknownType, canonical, strings.Join(ValidTypes(), ", "))
	}
	return canonical, nil
}

// EnabledTypes returns the sorted list of canonical type names that are
// both present in c.Types and have Enabled == true. The result is sorted
// alphabetically by canonical name for deterministic iteration in
// scaffolding and update flows.
func (c *Config) EnabledTypes() []string {
	enabled := make([]string, 0, len(c.Types))
	for name, tc := range c.Types {
		if tc.Enabled {
			enabled = append(enabled, name)
		}
	}
	sort.Strings(enabled)
	return enabled
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
			warnings = append(warnings,
				fmt.Sprintf("config declares non-built-in type %q (typo?)", name))
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
	fillTypeFieldDefaults(&cfg)

	return cfg, nil
}

// fillTypeFieldDefaults backfills zero-valued string, int, and slice
// fields on each cfg.Types entry from the corresponding DefaultConfig()
// entry. This works around an mapstructure behavior: for
// `map[string]TypeConfig` fields, the decoder allocates a fresh
// TypeConfig per key in the source rather than decoding in place over
// the pre-populated entry, so any field absent from the user's YAML
// is left at the zero value instead of inheriting the default.
//
// Bool fields (notably Enabled) are intentionally NOT filled here:
// the YAML decoder cannot distinguish "omitted" from "explicit false"
// for bools, so backfilling would silently re-enable a type the user
// disabled. Users must set `enabled: false` explicitly per type.
//
// For slice fields the distinguisher is nil-vs-empty: an omitted
// `statuses:` key decodes to a nil slice and IS filled from defaults;
// an explicit `statuses: []` decodes to a non-nil zero-length slice
// and is left alone so Validate can flag it.
//
// Custom types (entries not in DefaultConfig) are skipped — they have
// no defaults to draw from.
func fillTypeFieldDefaults(cfg *Config) {
	defaults := DefaultConfig()
	for name, tc := range cfg.Types {
		dtc, ok := defaults.Types[name]
		if !ok {
			continue
		}
		dstV := reflect.ValueOf(&tc).Elem()
		srcV := reflect.ValueOf(dtc)
		for i := 0; i < dstV.NumField(); i++ {
			f := dstV.Field(i)
			s := srcV.Field(i)
			switch f.Kind() {
			case reflect.String:
				if f.String() == "" {
					f.SetString(s.String())
				}
			case reflect.Int, reflect.Int64:
				if f.Int() == 0 {
					f.SetInt(s.Int())
				}
			case reflect.Slice:
				if f.IsNil() {
					f.Set(s)
				}
			}
		}
		cfg.Types[name] = tc
	}
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
