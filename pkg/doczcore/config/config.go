// Package config provides configuration loading and merging for docz.
//
// It is part of the public docz core (pkg/doczcore): both the docz CLI and
// external consumers such as docz-api import Load, Validate, the type
// resolution helpers (EnabledTypes, TypeDir, ValidateType), and the Config /
// TypeConfig shapes to read a repo's .docz.yaml identically. This surface is
// semver-governed (DESIGN-0007): adding fields is non-breaking; renaming or
// removing an exported symbol is a major change.
//
// A few exported helpers exist to serve the docz CLI's presentation layer
// rather than general consumers: TypesHelp (the `docz --help` body),
// DefaultNavTitles (MkDocs nav titles), and ResolveTypeAlias (bare
// registry-alias lookup; most callers want Config.ValidateType, which also
// honors per-type aliases and id_prefix resolution). They are public and
// semver-governed like everything else, but external consumers rarely need
// them (ADR-0001 Decisions).
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

// ChangelogConfig maps the changelog: block of .docz.yaml, which opts a
// repo into changelog awareness (DESIGN-0010). docz itself only locates
// the file and parses it (document.ParseChangelog) — generation belongs
// to git-cliff, and serving belongs to docz-api.
//
// The block carries yaml tags only: the mapstructure tags on its sibling
// blocks are vestigial, left from the viper era that ended in IMPL-0014
// Phase 1, and are not worth propagating to new code.
type ChangelogConfig struct {
	// Enabled opts the repo into changelog mapping. Default false, so
	// the block is dormant — and its File is left unvalidated — until a
	// repo turns it on.
	Enabled bool `yaml:"enabled"`

	// File is the changelog path relative to the repo root. Subpaths are
	// allowed for per-chart changelogs (charts/<name>/CHANGELOG.md).
	// Default "CHANGELOG.md"; an empty value resolves back to that
	// default at load time.
	File string `yaml:"file"`
}

// APIConfig declares what docz-api ingests and docz-site renders for
// this repo beyond the docz documents under the type directories
// (DESIGN-0011).
//
// The block is inert to the docz CLI: no command reads it. It exists to
// be a validated, semver-governed declaration that consumers read, which
// is why the paths here are checked as strictly as changelog.file — a
// consumer fetches them straight out of a git tree.
//
// The governing rule the fields encode: the URL path mirrors the
// docs_dir path. Every .md under docs_dir is consumable, a directory's
// README.md is that directory's page, and AdditionalDocs is the escape
// hatch for markdown that convention places outside docs_dir entirely.
//
// There is deliberately no nested index struct. Naming a landing page is
// enabling it, so a separate boolean would be redundant, and a nested
// api.index would collide with the top-level index: block that governs
// README table generation (Decision 2).
type APIConfig struct {
	// Enabled opts the repo into the api surface. Default false, so the
	// block is dormant — and its paths left unvalidated — until a repo
	// turns it on.
	//
	// This gates the *additional* surface only. A disabled or absent
	// block means a consumer ingests exactly what it does today: docz
	// documents under the type directories. It is not a switch for
	// ingesting the repo at all (Decision 4), because every existing
	// repo has no api: block and none of them should go dark.
	Enabled bool `yaml:"enabled"`

	// LandingPage is the repo's landing page, relative to the repo root.
	// Empty resolves to <docs_dir>/index.md at load time, so it tracks a
	// non-default docs_dir (Decision 3). Consumers address it as the repo
	// root rather than at its own path.
	LandingPage string `yaml:"landing_page"`

	// Exclude lists path prefixes under docs_dir that are never
	// published. Entries name directories, so a trailing "/" is
	// accepted and means the same thing without it.
	//
	// <docs_dir>/templates/ is always excluded regardless of this list —
	// it holds docz's own template overrides, which are machinery rather
	// than documents. Keeping that implicit is what lets this default to
	// empty, so a repo that sets Exclude does not silently lose the
	// protection (the footgun WikiConfig.Exclude has, where setting the
	// key replaces the default list wholesale).
	Exclude []string `yaml:"exclude"`

	// AdditionalDocs lists markdown OUTSIDE docs_dir, relative to the
	// repo root — CONTRIBUTING.md, DEVELOPMENT.md. Anything under
	// docs_dir is already consumed, so an entry there is a validation
	// error rather than a second record for one file.
	//
	// Bare strings rather than objects (Decision 1): the title comes
	// from the document's H1 via docparse.Title, and there is no
	// per-entry metadata worth the schema.
	AdditionalDocs []string `yaml:"additional_docs"`
}

// Config is the top-level configuration for docz.
type Config struct {
	DocsDir   string                `mapstructure:"docs_dir" yaml:"docs_dir"`
	Types     map[string]TypeConfig `mapstructure:"types"    yaml:"types"`
	Index     IndexConfig           `mapstructure:"index"    yaml:"index"`
	Author    AuthorConfig          `mapstructure:"author"   yaml:"author"`
	Wiki      WikiConfig            `mapstructure:"wiki"     yaml:"wiki"`
	TOC       TOCConfig             `mapstructure:"toc"      yaml:"toc"`
	Changelog ChangelogConfig       `                        yaml:"changelog"`
	API       APIConfig             `                        yaml:"api"`
}

// DefaultConfig returns the built-in default configuration. The per-type
// metadata (Types and Wiki.NavTitles) is sourced from the DocType
// registry in doctype.go so adding a new doc type is a single-file edit.
//
// Every built-in type is present in Types, but not every one is enabled:
// "plan" ships with Enabled false. Callers that want the effective set
// should use Config.EnabledTypes rather than ranging over Types.
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
		Changelog: ChangelogConfig{
			Enabled: false,
			File:    DefaultChangelogFile,
		},
		// Dormant, and deliberately zero-valued beyond that: LandingPage
		// is backfilled at load time so it can follow a non-default
		// docs_dir, and both slices stay nil so an empty list in a
		// config is distinguishable from an absent key.
		API: APIConfig{
			Enabled: false,
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

	var settings map[string]any

	// Load global config first.
	if home, err := os.UserHomeDir(); err == nil {
		globalSettings, mergeErr := readConfigMap(filepath.Join(home, ConfigFileName))
		if mergeErr != nil {
			return cfg, mergeErr
		}
		settings = mergeMaps(settings, globalSettings)
	}

	// Load repo-root config on top (deep merge, repo wins).
	repoSettings, mergeErr := readConfigMap(repoConfigPath)
	if mergeErr != nil {
		return cfg, mergeErr
	}
	settings = mergeMaps(settings, repoSettings)

	if err := decodeSettings(settings, &cfg); err != nil {
		return cfg, err
	}

	applyTypesReplaceOnPresence(&cfg, repoConfigPath)
	fillTypeFieldDefaults(&cfg)
	normalizeChangelog(&cfg)
	normalizeAPI(&cfg)

	return cfg, nil
}

// normalizeChangelog resolves ChangelogConfig.File to a canonical form:
// an explicitly empty value falls back to the default, and a leading
// "./" is stripped so "./CHANGELOG.md" and "CHANGELOG.md" are the same
// path to every consumer (DESIGN-0010).
//
// This is deliberately new machinery rather than a reuse of
// fillTypeFieldDefaults: that helper exists only because `Types` is a
// map whose values the decoder allocates fresh per key. Changelog is a
// plain struct field decoded in place over DefaultConfig(), so an
// omitted `file:` already inherits the default for free — only an
// explicit `file: ""` needs backfilling. Load normalizes; Validate only
// judges.
func normalizeChangelog(cfg *Config) {
	file := normalizeRepoPath(cfg.Changelog.File)
	if file == "" {
		file = DefaultChangelogFile
	}

	cfg.Changelog.File = file
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
//
// Note this is narrower than DocTypeNames: a built-in may ship disabled
// ("plan" does), so the default result is a subset of the registry.
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

	if err := c.validateChangelog(); err != nil {
		return warnings, err
	}

	return warnings, nil
}

// ErrInvalidChangelogFile is the sentinel wrapped by every
// changelog.file validation failure, so a consumer can tell a bad
// changelog path from any other config problem without matching on
// error text. Match it with errors.Is; the message carries the detail.
//
// Its text names the offending key so the wrapped message does not have
// to repeat it: the rendered chain reads
// `invalid changelog.file: "…" must not traverse outside the repo root`.
var ErrInvalidChangelogFile = errors.New("invalid changelog.file")

// validateChangelog rejects a changelog file path that consumers could
// not safely fetch out of a git tree (DESIGN-0010 Decision 5). The rules
// live in validateRepoRelativePath, shared with the api block so there is
// one hardening history rather than two that drift; see
// ErrInvalidChangelogFile for the shape of the rendered message.
//
// The check runs only for an enabled block (Decision 7). A repo may
// carry a dormant changelog: block — while rolling the feature out, or
// mid-edit — and a disabled block must never fail config load; the path
// is judged at the moment it starts being used.
func (c *Config) validateChangelog() error {
	if !c.Changelog.Enabled {
		return nil
	}

	if c.Changelog.File == "" {
		// Unreachable via Load (normalizeChangelog backfills the
		// default), but reachable for a hand-built Config. Checked here
		// rather than left to the shared helper so the message can say
		// what makes an empty value wrong in this block.
		return fmt.Errorf("%w: must not be empty when changelog is enabled",
			ErrInvalidChangelogFile)
	}

	// field is "" because ErrInvalidChangelogFile already names the key;
	// passing it would make the rendered chain stutter.
	if err := validateRepoRelativeFile("", c.Changelog.File); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidChangelogFile, err)
	}

	return nil
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

// readConfigMap reads a YAML config file into a raw settings map. A missing
// file is treated as "not configured" and returns an empty map. Anything else
// (permission denied, malformed YAML, etc.) is surfaced as a wrapped error
// so the user sees a clear message instead of a silently half-defaulted
// config — see IMPL-0006 Phase 4. Keys are matched case-sensitively by the
// yaml decoder downstream — the documented v1.0.0 decode delta vs the old
// case-insensitive viper loader (IMPL-0014 Phase 1).
func readConfigMap(path string) (map[string]any, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("checking config file %s: %w", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", path, err)
	}
	var settings map[string]any
	if err := yaml.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", path, err)
	}
	return settings, nil
}

// mergeMaps deep-merges src into dst and returns the result (src wins on
// conflicts). Nested string-keyed maps merge key-by-key; scalars and slices
// replace wholesale. This mirrors the semantics of viper's MergeConfigMap,
// which Load relied on before the yaml.v3 port, so global-config +
// repo-config precedence is unchanged.
func mergeMaps(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = make(map[string]any, len(src))
	}
	for key, srcVal := range src {
		if dstMap, ok := dst[key].(map[string]any); ok {
			if srcMap, ok := srcVal.(map[string]any); ok {
				dst[key] = mergeMaps(dstMap, srcMap)
				continue
			}
		}
		dst[key] = srcVal
	}
	return dst
}

// decodeSettings decodes a merged raw settings map onto cfg by
// round-tripping it through yaml, so a partial config only touches the keys
// it names and every sibling default on the pre-populated cfg survives (the
// IMPL-0006 Phase 2 contract). Unknown keys are ignored (viper-parity
// leniency — IMPL-0014 Decision 2).
func decodeSettings(settings map[string]any, cfg *Config) error {
	if len(settings) == 0 {
		return nil
	}
	data, err := yaml.Marshal(settings)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, cfg)
}

func loadFromFile(path string, defaults *Config) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return *defaults, err
	}

	cfg := *defaults
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return *defaults, err
	}

	applyTypesReplaceOnPresence(&cfg, path)
	fillTypeFieldDefaults(&cfg)
	normalizeChangelog(&cfg)
	normalizeAPI(&cfg)

	return cfg, nil
}

// fillTypeFieldDefaults backfills zero-valued string, int, and slice
// fields on each cfg.Types entry from the corresponding DefaultConfig()
// entry. This works around the decoder's map handling (yaml.v3 today,
// mapstructure before the IMPL-0014 viper removal — both behave the same
// way): for `map[string]TypeConfig` fields, the decoder allocates a fresh
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
