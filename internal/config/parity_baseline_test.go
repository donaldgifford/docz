package config_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"text/template"

	"go.yaml.in/yaml/v3"

	"github.com/donaldgifford/docz/internal/config"
	doctemplate "github.com/donaldgifford/docz/internal/template"
)

// TestDoczYAMLTemplate_RoundTripsToDefaultConfig is the IMPL-0006 Phase 1
// regression guard: the embedded `.docz.yaml.tmpl` rendered with
// DefaultConfig() and parsed back must equal DefaultConfig(). Catches
// drift between the template, DefaultConfig, and the YAML schema if
// any of the three change without the others.
func TestDoczYAMLTemplate_RoundTripsToDefaultConfig(t *testing.T) {
	tmplSrc, err := doctemplate.EmbeddedDoczYAML()
	if err != nil {
		t.Fatalf("loading template: %v", err)
	}

	tmpl, err := template.New("docz_yaml").Parse(tmplSrc)
	if err != nil {
		t.Fatalf("parsing template: %v", err)
	}

	want := config.DefaultConfig()
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, want); err != nil {
		t.Fatalf("rendering template: %v", err)
	}

	var got config.Config
	if err := yaml.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshalling rendered yaml: %v\nrendered:\n%s", err, buf.String())
	}

	if !reflect.DeepEqual(want, got) {
		t.Errorf("rendered template did not round-trip to DefaultConfig()\nwant: %#v\ngot:  %#v", want, got)
	}
}

// TestDoczYAMLTemplate_RetainsCommentHeader guards the human-readable
// header block at the top of the rendered file. Pure yaml.Marshal would
// drop comments; the template approach preserves them. If a future change
// switches back to marshal, this catches it.
func TestDoczYAMLTemplate_RetainsCommentHeader(t *testing.T) {
	tmplSrc, err := doctemplate.EmbeddedDoczYAML()
	if err != nil {
		t.Fatalf("loading template: %v", err)
	}

	tmpl, err := template.New("docz_yaml").Parse(tmplSrc)
	if err != nil {
		t.Fatalf("parsing template: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config.DefaultConfig()); err != nil {
		t.Fatalf("rendering template: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"# .docz.yaml -- configuration for the docz CLI",
		"# About the `types:` block",
		"# Documentation: https://github.com/donaldgifford/docz",
	} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("rendered output missing header line %q", want)
		}
	}
}

// TestLoad_PartialOverridesPreserveSiblingDefaults is the IMPL-0006 Phase 2
// regression guard. With `setDefaults` removed, the only thing that keeps a
// user's partial config from clobbering sibling defaults is that
// `Load`/`loadFromFile` unmarshal viper output onto a pre-populated
// `DefaultConfig()` (so mapstructure leaves untouched fields alone). If a
// future refactor reintroduces a `var cfg Config` zero-init, these checks
// catch it.
func TestLoad_PartialOverridesPreserveSiblingDefaults(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "partial.yaml")
	content := `wiki:
  repo_url: https://example.com/repo
toc:
  enabled: false
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	defaults := config.DefaultConfig()

	if cfg.Wiki.RepoURL != "https://example.com/repo" {
		t.Errorf("Wiki.RepoURL = %q, want override", cfg.Wiki.RepoURL)
	}
	if cfg.Wiki.MkDocsPath != defaults.Wiki.MkDocsPath {
		t.Errorf(
			"Wiki.MkDocsPath = %q, want default %q",
			cfg.Wiki.MkDocsPath, defaults.Wiki.MkDocsPath,
		)
	}
	if !cfg.Wiki.AutoUpdate {
		t.Error("Wiki.AutoUpdate lost when only repo_url set")
	}
	if !reflect.DeepEqual(cfg.Wiki.Plugins, defaults.Wiki.Plugins) {
		t.Errorf("Wiki.Plugins = %v, want default %v", cfg.Wiki.Plugins, defaults.Wiki.Plugins)
	}

	if cfg.ToC.Enabled {
		t.Error("ToC.Enabled override not applied")
	}
	if cfg.ToC.MinHeadings != defaults.ToC.MinHeadings {
		t.Errorf(
			"ToC.MinHeadings = %d, want default %d",
			cfg.ToC.MinHeadings, defaults.ToC.MinHeadings,
		)
	}

	if len(cfg.Types) != len(defaults.Types) {
		t.Errorf("Types count = %d, want %d (defaults preserved)",
			len(cfg.Types), len(defaults.Types))
	}
}

// TestLoad_RepoConfigPartialOverridesPreserveSiblingDefaults covers the
// repo-root config path (not explicit-file), which uses MergeConfigMap +
// Unmarshal-on-defaults instead of loadFromFile.
func TestLoad_RepoConfigPartialOverridesPreserveSiblingDefaults(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	content := `author:
  default: "Test Author"
`
	if err := os.WriteFile(".docz.yaml", []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	defaults := config.DefaultConfig()

	if cfg.Author.Default != "Test Author" {
		t.Errorf("Author.Default = %q, want override", cfg.Author.Default)
	}
	if !cfg.Author.FromGit {
		t.Error("Author.FromGit lost when only default set")
	}
	if cfg.DocsDir != defaults.DocsDir {
		t.Errorf("DocsDir = %q, want default %q", cfg.DocsDir, defaults.DocsDir)
	}
	if !reflect.DeepEqual(cfg.Wiki, defaults.Wiki) {
		t.Errorf("Wiki section lost defaults: got %#v, want %#v", cfg.Wiki, defaults.Wiki)
	}
}

// TestLoad_MalformedRepoConfigReturnsError is the IMPL-0006 Phase 4
// regression guard: a .docz.yaml that fails to parse must now surface a
// wrapped error with the file path in the message, instead of being
// silently swallowed in mergeConfigFile.
func TestLoad_MalformedRepoConfigReturnsError(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// Tab inside a value plus an unclosed brace -> YAML parse error.
	content := "types: {rfc: {dir: foo\n  not valid: : :"
	if err := os.WriteFile(".docz.yaml", []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, loadErr := config.Load("")
	if loadErr == nil {
		t.Fatal("expected parse error from malformed .docz.yaml, got nil")
	}
	msg := loadErr.Error()
	if !bytes.Contains([]byte(msg), []byte("parsing config file")) {
		t.Errorf("error %q missing %q prefix", msg, "parsing config file")
	}
	if !bytes.Contains([]byte(msg), []byte(".docz.yaml")) {
		t.Errorf("error %q missing file path", msg)
	}
}

// TestLoad_MissingRepoConfigSilent is the companion guard: a missing
// .docz.yaml must continue to return defaults without error. This is the
// green-field case (new repo, no config yet) and must keep working.
func TestLoad_MissingRepoConfigSilent(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	cfg, loadErr := config.Load("")
	if loadErr != nil {
		t.Fatalf("missing .docz.yaml should not error, got %v", loadErr)
	}
	if cfg.DocsDir != "docs" {
		t.Errorf("DocsDir = %q, want defaults", cfg.DocsDir)
	}
}

// TestLoad_UnreadableRepoConfigReturnsError covers the third Phase 4 case:
// a .docz.yaml that exists but cannot be read (mode 0000) must surface a
// wrapped error rather than be silently swallowed.
//
// Skipped when running as root (CI containers often do) because root
// bypasses permission bits.
func TestLoad_UnreadableRepoConfigReturnsError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission bits don't constrain root; skipping unreadable-file test")
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(".docz.yaml", []byte("docs_dir: docs\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	// Make sure the file gets cleaned up even though it has mode 0000.
	t.Cleanup(func() { _ = os.Chmod(".docz.yaml", 0o644) })

	_, loadErr := config.Load("")
	if loadErr == nil {
		t.Fatal("expected error reading unreadable .docz.yaml, got nil")
	}
	if !bytes.Contains([]byte(loadErr.Error()), []byte(".docz.yaml")) {
		t.Errorf("error %q missing file path", loadErr.Error())
	}
}

// TestLoad_TypeFieldDefaultsBackfilled is the IMPL-0006 Phase 8
// regression guard. mapstructure's map-of-struct decoding allocates a
// fresh TypeConfig per source key, so any field absent from the user's
// YAML is left at the zero value rather than inheriting the default —
// the F49 bug for the Types map. fillTypeFieldDefaults closes that gap
// for string, int, and (nil) slice fields; this test pins the
// PluralLabel + IDPrefix + Statuses backfill so a future refactor
// can't regress it.
//
// Bool fields (Enabled) are intentionally NOT backfilled and are
// covered by a sibling subtest.
func TestLoad_TypeFieldDefaultsBackfilled(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "partial.yaml")
	// User declares rfc with only a custom dir — everything else
	// (id_prefix, id_width, statuses, status_field, plural_label) is
	// omitted and should be filled from DefaultConfig().
	content := `types:
  rfc:
    enabled: true
    dir: rfc-custom
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	tc, ok := cfg.Types["rfc"]
	if !ok {
		t.Fatal("rfc type missing after partial config")
	}
	if tc.Dir != "rfc-custom" {
		t.Errorf("Dir = %q, want override", tc.Dir)
	}
	defaults := config.DefaultConfig()
	dtc := defaults.Types["rfc"]
	if tc.IDPrefix != dtc.IDPrefix {
		t.Errorf("IDPrefix = %q, want default %q", tc.IDPrefix, dtc.IDPrefix)
	}
	if tc.IDWidth != dtc.IDWidth {
		t.Errorf("IDWidth = %d, want default %d", tc.IDWidth, dtc.IDWidth)
	}
	if tc.PluralLabel != dtc.PluralLabel {
		t.Errorf("PluralLabel = %q, want default %q", tc.PluralLabel, dtc.PluralLabel)
	}
	if tc.StatusField != dtc.StatusField {
		t.Errorf("StatusField = %q, want default %q", tc.StatusField, dtc.StatusField)
	}
	if !reflect.DeepEqual(tc.Statuses, dtc.Statuses) {
		t.Errorf("Statuses = %v, want default %v", tc.Statuses, dtc.Statuses)
	}
}

// TestLoad_TypeExplicitEmptyStatusesPreserved guards the nil-vs-empty
// distinction for slice fields: `statuses: []` in YAML must NOT be
// backfilled from defaults, so Validate can still flag the type as
// misconfigured.
func TestLoad_TypeExplicitEmptyStatusesPreserved(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "explicit-empty.yaml")
	content := `types:
  rfc:
    enabled: true
    dir: rfc
    statuses: []
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Types["rfc"].Statuses; len(got) != 0 {
		t.Errorf("Statuses = %v, want explicit empty preserved", got)
	}
	if _, err := cfg.Validate(); err == nil {
		t.Error("Validate should error on explicit empty statuses")
	}
}

// TestLoad_DefaultsParity is the IMPL-0006 Phase 2 reflective parity guard.
// With no config files present, Load() must return a Config deep-equal to
// DefaultConfig(). Catches any future drift where Load loses or mutates
// fields relative to DefaultConfig.
func TestLoad_DefaultsParity(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	got, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := config.DefaultConfig()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load() with no config files diverged from DefaultConfig()\nwant: %#v\ngot:  %#v",
			want, got)
	}
}
