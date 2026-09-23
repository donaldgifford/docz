package ingest

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestConfigLoadsFixtureManifest is the one clause of the retired
// internal/doczcontract package that still earns its keep once docz-api and
// docz share a module (ADR-0004 OQ 3, IMPL-0019 Phase 3): the manifest shape
// ingest reads — built-in types plus a custom "frameworks" type with an
// id_prefix and an alias — decodes and validates through loadConfig. The rest
// of the contract is pinned by pkg/doczcore's own tests.
func TestConfigLoadsFixtureManifest(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("testdata", "manifest.docz.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(raw)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if warnings, err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v (warnings: %v)", err, warnings)
	}

	if cfg.DocsDir != "docs" {
		t.Errorf("DocsDir = %q, want %q", cfg.DocsDir, "docs")
	}
	if got, want := cfg.TypeDir("frameworks"), filepath.Join("docs", "frameworks"); got != want {
		t.Errorf("TypeDir(frameworks) = %q, want %q", got, want)
	}
	if enabled := cfg.EnabledTypes(); !slices.Contains(enabled, "frameworks") {
		t.Errorf("EnabledTypes() = %v, want it to include %q", enabled, "frameworks")
	}
}
