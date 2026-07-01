// Package config provides configuration loading and merging for docz.
//
// It is part of the public docz core (pkg/doczcore): both the docz CLI and
// external consumers such as docz-api import Load, Validate, the type
// resolution helpers (EnabledTypes, TypeDir, ValidateType), and the Config /
// TypeConfig shapes to read a repo's .docz.yaml identically. This surface is
// semver-governed (DESIGN-0007): adding fields is non-breaking; renaming or
// removing an exported symbol is a major change.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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
	// Aliases are optional per-type CLI shorthands (e.g. "fw" for a
	// "frameworks" type), resolved by resolveType alongside the built-in
	// registry aliases (DESIGN-0006 Decision 6). Empty for the built-ins,
	// whose aliases live in the DocType registry.
	Aliases []string `mapstructure:"aliases" yaml:"aliases,omitempty"`
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

// TOCConfig holds configuration for table of contents generation.
//
// The YAML/mapstructure tags deliberately stay as "toc" so existing
// .docz.yaml files keep working unchanged after the Go-side rename
// (`ToC` → `TOC`) — see IMPL-0008 Decisions §5.
type TOCConfig struct {
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
	TOC     TOCConfig             `mapstructure:"toc"      yaml:"toc"`
}

// DefaultConfig returns the built-in default configuration. The per-type
// metadata (Types and Wiki.NavTitles) is sourced from the DocType
// registry in doctype.go so adding a new doc type is a single-file edit.
func DefaultConfig() Config {
	return Config{
		DocsDir: "docs",
		Types:   defaultTypesMap(),
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
			NavTitles:  defaultNavTitlesMap(),
		},
		TOC: TOCConfig{
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
//
// repoRoot is the directory to search for ConfigFileName when configFile
// is empty. An empty repoRoot falls back to the current working
// directory for backwards compatibility with callers that have not yet
// been updated; new callers (cmd/root.go since IMPL-0009 Phase 7)
// should pass an explicit path so tests can scope config discovery to
// a t.TempDir() without os.Chdir.
func Load(configFile, repoRoot string) (Config, error) {
	cfg := DefaultConfig()

	if configFile != "" {
		return loadFromFile(configFile, &cfg)
	}

	repoConfigPath := ConfigFileName
	if repoRoot != "" {
		repoConfigPath = filepath.Join(repoRoot, ConfigFileName)
	}

	v := viper.New()

	// Load global config first.
	if home, err := os.UserHomeDir(); err == nil {
		if mergeErr := mergeConfigFile(v, filepath.Join(home, ConfigFileName)); mergeErr != nil {
			return cfg, mergeErr
		}
	}

	// Load repo-root config on top (deep merge, repo wins).
	if mergeErr := mergeConfigFile(v, repoConfigPath); mergeErr != nil {
		return cfg, mergeErr
	}

	if err := v.Unmarshal(&cfg); err != nil {
		return cfg, err
	}

	applyTypesReplaceOnPresence(&cfg, repoConfigPath)
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
// docz-managed type directories, sourced from the DocType registry.
func DefaultNavTitles() map[string]string {
	return defaultNavTitlesMap()
}

// ErrUnknownType is the sentinel returned by ValidateType when the input
// does not name a built-in document type. Callers can branch on it with
// errors.Is to render a custom hint without parsing the wrapped message.
var ErrUnknownType = errors.New("unknown document type")

// ValidateType canonicalizes and validates a user-supplied type name via
// resolveType (canonical name → alias → id_prefix, case-insensitive). On
// success it returns the canonical Config.Types key; on failure it returns
// a fmt.Errorf-wrapped ErrUnknownType listing the enabled types.
//
// Callers that need the canonical name and want a single error site
// should use this helper instead of duplicating the lookup-and-format
// block at each CLI subcommand boundary (IMPL-0006 Phase 7).
func (c *Config) ValidateType(name string) (string, error) {
	if canonical, ok := c.resolveType(name); ok {
		return canonical, nil
	}
	// Quote name verbatim so the user sees what they typed, not the
	// normalized form resolveType matched against.
	return "", fmt.Errorf("%w %q (valid types: %s)",
		ErrUnknownType, name, strings.Join(c.EnabledTypes(), ", "))
}

// resolveType maps a user-supplied token to a canonical Config.Types key,
// case-insensitively, in precedence order: canonical name, then alias (a
// built-in registry alias such as "inv", or a per-type Aliases entry), then
// id_prefix (so "FW"/"fw" resolve the type whose id_prefix is "FW"). ok is
// false when nothing matches. Name beats alias beats prefix, so a prefix or
// alias can never shadow a real type name. Ambiguous aliases/prefixes across
// types are rejected by Validate (DESIGN-0006 Decision 5), so at most one
// match is expected at the alias and prefix tiers.
func (c *Config) resolveType(name string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(name))

	// 1. Canonical name.
	if _, ok := c.Types[lower]; ok {
		return lower, true
	}

	// 2a. Built-in registry alias (e.g. "inv" -> "investigation").
	if canonical := ResolveTypeAlias(lower); canonical != lower {
		if _, ok := c.Types[canonical]; ok {
			return canonical, true
		}
	}

	// 2b. Per-type alias declared in .docz.yaml. Range keys only —
	// TypeConfig is heavy, so a value-copy range trips gocritic.
	for key := range c.Types {
		for _, alias := range c.Types[key].Aliases {
			if strings.EqualFold(alias, lower) {
				return key, true
			}
		}
	}

	// 3. id_prefix.
	for key := range c.Types {
		if prefix := c.Types[key].IDPrefix; prefix != "" && strings.EqualFold(prefix, lower) {
			return key, true
		}
	}

	return "", false
}

// EnabledTypes returns the canonical names of every enabled type in c.Types:
// the built-ins first in DocType-registry declaration order, then any enabled
// custom types (those not in the registry) sorted alphabetically. The two-part
// order is deterministic — built-ins keep their familiar order and custom
// types get a stable sort, since Go map iteration is unordered (IMPL-0012
// Phase 4, Decision 1). Including custom types here is what lets no-argument
// commands (docz update / init / list / wiki) scaffold and iterate them.
func (c *Config) EnabledTypes() []string {
	enabled := make([]string, 0, len(c.Types))
	builtin := make(map[string]bool, len(DocTypeNames()))
	for _, name := range DocTypeNames() {
		builtin[name] = true
		if c.Types[name].Enabled {
			enabled = append(enabled, name)
		}
	}

	custom := make([]string, 0, len(c.Types))
	for name := range c.Types {
		if !builtin[name] && c.Types[name].Enabled {
			custom = append(custom, name)
		}
	}
	slices.Sort(custom)

	return append(enabled, custom...)
}

// typeAliases maps short or alternate names to their canonical type name.
// Sourced from the DocType registry's Aliases entries.
var typeAliases = defaultTypeAliases()

// ResolveTypeAlias returns the canonical type name for the given input.
// If the input is already a canonical name or has no alias, it is returned as-is.
func ResolveTypeAlias(name string) string {
	if canonical, ok := typeAliases[name]; ok {
		return canonical
	}
	return name
}

// TypesHelp returns a formatted help string listing all valid types
// with aliases. The body is derived from the DocType registry —
// adding a new entry to `allDocTypes` with a `HelpDescription` is
// the only step required to surface it in `docz --help`.
func TypesHelp() string {
	const nameColWidth = 17

	var b strings.Builder
	b.WriteString("Document types:")
	for _, dt := range allDocTypes {
		b.WriteString("\n  ")
		b.WriteString(dt.Name)
		for i := len(dt.Name); i < nameColWidth; i++ {
			b.WriteByte(' ')
		}
		b.WriteString(dt.HelpDescription)
		if len(dt.Aliases) > 0 {
			b.WriteString(" (alias: ")
			b.WriteString(strings.Join(dt.Aliases, ", "))
			b.WriteByte(')')
		}
	}
	return b.String()
}

// Validate checks the configuration for common errors and returns a list of
// warnings and the first error found (if any).
func (c *Config) Validate() ([]string, error) {
	var warnings []string

	if c.DocsDir == "" {
		return warnings, errors.New("docs_dir must not be empty")
	}

	validTypes := map[string]bool{}
	for _, t := range DocTypeNames() {
		validTypes[t] = true
	}

	for name := range c.Types {
		if !validTypes[name] {
			warnings = append(warnings,
				fmt.Sprintf("config declares non-built-in type %q (typo?)", name))
		}
		if c.Types[name].Enabled && len(c.Types[name].Statuses) == 0 {
			return warnings, fmt.Errorf("type %q has no statuses defined", name)
		}
	}

	if err := c.validateResolution(); err != nil {
		return warnings, err
	}

	return warnings, nil
}

// validateResolution rejects configs where two enabled types could resolve
// from the same token, which would make resolveType ambiguous (and Go's
// unordered map iteration nondeterministic). The collision domain is the
// union, over enabled types, of {canonical name, built-in registry alias,
// per-type alias, id_prefix}, matched case-insensitively (IMPL-0012 Phase 4,
// DESIGN-0006 Decision 5). A token claimed twice by the same type (e.g. a
// built-in whose name and id_prefix lower-case to the same value) is fine;
// only cross-type duplicates are errors.
func (c *Config) validateResolution() error {
	seen := make(map[string]string) // resolution token -> owning type key
	claim := func(token, owner, kind string) error {
		t := strings.ToLower(strings.TrimSpace(token))
		if t == "" {
			return nil
		}
		if prev, ok := seen[t]; ok && prev != owner {
			return fmt.Errorf(
				"type %q %s %q collides with type %q: resolution would be ambiguous",
				owner, kind, token, prev)
		}
		seen[t] = owner
		return nil
	}

	enabledList := c.EnabledTypes()
	enabled := make(map[string]bool, len(enabledList))
	for _, name := range enabledList {
		enabled[name] = true
	}

	for _, name := range enabledList {
		if err := claim(name, name, "name"); err != nil {
			return err
		}
		for _, alias := range c.Types[name].Aliases {
			if err := claim(alias, name, "alias"); err != nil {
				return err
			}
		}
		if err := claim(c.Types[name].IDPrefix, name, "id_prefix"); err != nil {
			return err
		}
	}

	// Built-in registry aliases (e.g. "inv", "implementation") for enabled
	// built-ins — resolveType consults these in tier 2a, so a custom alias
	// or prefix that shadows one is the same class of ambiguity.
	for _, dt := range allDocTypes {
		if !enabled[dt.Name] {
			continue
		}
		for _, alias := range dt.Aliases {
			if err := claim(alias, dt.Name, "registry alias"); err != nil {
				return err
			}
		}
	}

	return nil
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
	for name := range cfg.Types {
		dtc, ok := defaults.Types[name]
		if !ok {
			continue
		}
		// Explicit local copy (not a range-value copy) so we can mutate
		// via reflect and write back; TypeConfig is heavy enough that a
		// range-value copy trips gocritic's rangeValCopy.
		tc := cfg.Types[name]
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
