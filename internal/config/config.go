// Package config provides configuration loading and merging for docz.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
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
			MkDocsPath: "mkdocs.yml",
			Plugins:    []string{"techdocs-core"},
			Exclude:    []string{"templates", "examples"},
			NavTitles:  DefaultNavTitles(),
		},
		ToC: ToCConfig{
			Enabled:     true,
			MinHeadings: 3,
		},
	}
}

// Load reads configuration from the global (~/.docz.yaml) and repo-root
// (.docz.yaml) config files, deep-merging them with repo root taking
// precedence. Built-in defaults are applied for any missing keys.
//
// If configFile is non-empty, it is used as the sole config source (no merge).
func Load(configFile string) (Config, error) {
	cfg := DefaultConfig()

	if configFile != "" {
		return loadFromFile(configFile, &cfg)
	}

	v := viper.New()
	setDefaults(v, &cfg)

	// Load global config first.
	if home, err := os.UserHomeDir(); err == nil {
		if mergeErr := mergeConfigFile(v, filepath.Join(home, ".docz.yaml")); mergeErr != nil {
			return cfg, mergeErr
		}
	}

	// Load repo-root config on top (deep merge, repo wins).
	if mergeErr := mergeConfigFile(v, ".docz.yaml"); mergeErr != nil {
		return cfg, mergeErr
	}

	if err := v.Unmarshal(&cfg); err != nil {
		return cfg, err
	}

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
func (c *Config) Validate() (warnings []string, err error) {
	if c.DocsDir == "" {
		err = fmt.Errorf("docs_dir must not be empty")
		return warnings, err
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
			err = fmt.Errorf("type %q has no statuses defined", name)
			return warnings, err
		}
	}

	return warnings, nil
}

// mergeConfigFile reads a YAML config file and merges it into v. If the file
// does not exist or cannot be read, it is silently skipped.
func mergeConfigFile(v *viper.Viper, path string) error {
	if _, err := os.Stat(path); err != nil {
		return nil //nolint:nilerr // missing config file is not an error
	}
	fileV := viper.New()
	fileV.SetConfigFile(path)
	if err := fileV.ReadInConfig(); err != nil {
		return nil //nolint:nilerr // unreadable config file is silently skipped
	}
	return v.MergeConfigMap(fileV.AllSettings())
}

func loadFromFile(path string, defaults *Config) (Config, error) {
	v := viper.New()
	setDefaults(v, defaults)
	v.SetConfigFile(path)

	if err := v.ReadInConfig(); err != nil {
		return *defaults, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return *defaults, err
	}

	return cfg, nil
}

func setDefaults(v *viper.Viper, cfg *Config) {
	v.SetDefault("docs_dir", cfg.DocsDir)
	v.SetDefault("index.auto_update", cfg.Index.AutoUpdate)
	v.SetDefault("index.preserve_header", cfg.Index.PreserveHeader)
	v.SetDefault("author.from_git", cfg.Author.FromGit)
	v.SetDefault("author.default", cfg.Author.Default)

	for name, tc := range cfg.Types {
		prefix := "types." + name + "."
		v.SetDefault(prefix+"enabled", tc.Enabled)
		v.SetDefault(prefix+"dir", tc.Dir)
		v.SetDefault(prefix+"template", tc.Template)
		v.SetDefault(prefix+"id_prefix", tc.IDPrefix)
		v.SetDefault(prefix+"id_width", tc.IDWidth)
		v.SetDefault(prefix+"statuses", tc.Statuses)
		v.SetDefault(prefix+"status_field", tc.StatusField)
	}

	v.SetDefault("wiki.auto_update", cfg.Wiki.AutoUpdate)
	v.SetDefault("wiki.mkdocs_path", cfg.Wiki.MkDocsPath)
	v.SetDefault("wiki.plugins", cfg.Wiki.Plugins)
	v.SetDefault("wiki.exclude", cfg.Wiki.Exclude)
	for k, val := range cfg.Wiki.NavTitles {
		v.SetDefault("wiki.nav_titles."+k, val)
	}

	v.SetDefault("toc.enabled", cfg.ToC.Enabled)
	v.SetDefault("toc.min_headings", cfg.ToC.MinHeadings)
}
