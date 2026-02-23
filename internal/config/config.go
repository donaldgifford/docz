// Package config provides configuration loading and merging for docz.
package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// TypeConfig holds configuration for a single document type.
type TypeConfig struct {
	Enabled     bool     `mapstructure:"enabled"`
	Dir         string   `mapstructure:"dir"`
	Template    string   `mapstructure:"template"`
	IDPrefix    string   `mapstructure:"id_prefix"`
	IDWidth     int      `mapstructure:"id_width"`
	Statuses    []string `mapstructure:"statuses"`
	StatusField string   `mapstructure:"status_field"`
}

// IndexConfig holds configuration for index/README generation.
type IndexConfig struct {
	AutoUpdate     bool `mapstructure:"auto_update"`
	PreserveHeader bool `mapstructure:"preserve_header"`
}

// AuthorConfig holds configuration for author resolution.
type AuthorConfig struct {
	FromGit bool   `mapstructure:"from_git"`
	Default string `mapstructure:"default"`
}

// Config is the top-level configuration for docz.
type Config struct {
	DocsDir string                `mapstructure:"docs_dir"`
	Types   map[string]TypeConfig `mapstructure:"types"`
	Index   IndexConfig           `mapstructure:"index"`
	Author  AuthorConfig          `mapstructure:"author"`
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
		},
		Index: IndexConfig{
			AutoUpdate:     true,
			PreserveHeader: true,
		},
		Author: AuthorConfig{
			FromGit: true,
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
		return loadFromFile(configFile, cfg)
	}

	v := viper.New()
	setDefaults(v, cfg)

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

// ValidTypes returns the list of built-in document type names.
func ValidTypes() []string {
	return []string{"rfc", "adr", "design", "impl"}
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

func loadFromFile(path string, defaults Config) (Config, error) {
	v := viper.New()
	setDefaults(v, defaults)
	v.SetConfigFile(path)

	if err := v.ReadInConfig(); err != nil {
		return defaults, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return defaults, err
	}

	return cfg, nil
}

func setDefaults(v *viper.Viper, cfg Config) {
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
}
