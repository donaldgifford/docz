package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/donaldgifford/docz/internal/config"
)

// The five e2e tests below correspond one-to-one with the INV-0003
// scenario enumeration in docs/impl/0006 Phase 5. They go through the
// full rootCmd PreRunE -> subcommand pipeline (not direct runInit /
// runUpdate calls) so they exercise the same Load() pathway end users
// hit.

// setupINV0003Test writes the supplied config content (if any) into a
// fresh t.TempDir() and points `repoRoot` at it. The Cobra
// PersistentPreRunE then resolves the repo root from the flag rather
// than process cwd, so scaffolding lands under dir without any
// os.Chdir.
func setupINV0003Test(t *testing.T, configContent string) string {
	t.Helper()
	dir := t.TempDir()

	if configContent != "" {
		if err := os.WriteFile(
			filepath.Join(dir, ".docz.yaml"),
			[]byte(configContent),
			0o644,
		); err != nil {
			t.Fatal(err)
		}
	}
	cfgFile = ""
	repoRoot = dir
	docsDir = ""
	t.Cleanup(func() {
		cfgFile = ""
		repoRoot = ""
		docsDir = ""
		appCfg = config.DefaultConfig()
		runner = nil
	})
	return dir
}

func runRoot(t *testing.T, args ...string) {
	t.Helper()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs(args)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("docz %v failed: %v (output: %s)", args, err, buf.String())
	}
}

func TestINV0003_RFCOnlyConfig_InitScaffoldsOnlyRFC(t *testing.T) {
	dir := setupINV0003Test(t, `types:
  rfc:
    enabled: true
    dir: rfc
    id_prefix: RFC
    id_width: 4
    statuses: [Draft, Accepted]
    status_field: status
`)

	runRoot(t, "init")

	if _, err := os.Stat(filepath.Join(dir, "docs", "rfc")); err != nil {
		t.Errorf("docs/rfc should exist: %v", err)
	}

	for _, typeName := range []string{"adr", "design", "impl", "plan", "investigation"} {
		if _, err := os.Stat(filepath.Join(dir, "docs", typeName)); err == nil {
			t.Errorf("docs/%s must NOT be created when config only lists rfc", typeName)
		}
	}
}

func TestINV0003_RFCOnlyConfig_UpdateTouchesOnlyRFC(t *testing.T) {
	dir := setupINV0003Test(t, `types:
  rfc:
    enabled: true
    dir: rfc
    id_prefix: RFC
    id_width: 4
    statuses: [Draft, Accepted]
    status_field: status
`)

	runRoot(t, "init")
	// Touch sentinel files in directories that should NOT be revived.
	for _, typeName := range []string{"adr", "design"} {
		fullDir := filepath.Join(dir, "docs", typeName)
		if err := os.MkdirAll(fullDir, 0o755); err != nil {
			t.Fatal(err)
		}
		readme := filepath.Join(fullDir, "README.md")
		if err := os.WriteFile(readme, []byte("# untouched\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Snapshot the untouched README contents.
	type snap struct{ path, body string }
	sentinelDirs := []string{"adr", "design"}
	sentinels := make([]snap, 0, len(sentinelDirs))
	for _, typeName := range sentinelDirs {
		readme := filepath.Join(dir, "docs", typeName, "README.md")
		data, err := os.ReadFile(readme)
		if err != nil {
			t.Fatal(err)
		}
		sentinels = append(sentinels, snap{path: readme, body: string(data)})
	}

	runRoot(t, "update")

	for _, s := range sentinels {
		data, err := os.ReadFile(s.path)
		if err != nil {
			t.Fatalf("reading sentinel %s: %v", s.path, err)
		}
		if string(data) != s.body {
			t.Errorf("docz update mutated %s (config only lists rfc):\nbefore: %q\nafter:  %q",
				s.path, s.body, string(data))
		}
	}
}

func TestINV0003_NoConfig_InitScaffoldsAllSix(t *testing.T) {
	dir := setupINV0003Test(t, "")

	runRoot(t, "init")

	for _, typeName := range []string{"rfc", "adr", "design", "impl", "plan", "investigation"} {
		if _, err := os.Stat(filepath.Join(dir, "docs", typeName)); err != nil {
			t.Errorf("docs/%s should be scaffolded in green-field init: %v", typeName, err)
		}
	}
}

func TestINV0003_DisabledADRListed_OnlyRFCScaffolded(t *testing.T) {
	dir := setupINV0003Test(t, `types:
  rfc:
    enabled: true
    dir: rfc
    id_prefix: RFC
    id_width: 4
    statuses: [Draft, Accepted]
    status_field: status
  adr:
    enabled: false
    dir: adr
    id_prefix: ADR
    id_width: 4
    statuses: [Accepted]
    status_field: status
`)

	runRoot(t, "init")

	if _, err := os.Stat(filepath.Join(dir, "docs", "rfc")); err != nil {
		t.Errorf("docs/rfc should exist: %v", err)
	}
	// adr is listed but disabled -> still skipped by init's enabled gate.
	if _, err := os.Stat(filepath.Join(dir, "docs", "adr")); err == nil {
		t.Error("docs/adr should NOT be scaffolded when enabled:false")
	}
	for _, typeName := range []string{"design", "impl", "plan", "investigation"} {
		if _, err := os.Stat(filepath.Join(dir, "docs", typeName)); err == nil {
			t.Errorf("docs/%s should NOT be scaffolded (not listed in config)", typeName)
		}
	}
}

func TestINV0003_IncrementalAddType_PreservesExistingFiles(t *testing.T) {
	dir := setupINV0003Test(t, `types:
  rfc:
    enabled: true
    dir: rfc
    id_prefix: RFC
    id_width: 4
    statuses: [Draft, Accepted]
    status_field: status
`)

	runRoot(t, "init")

	// Capture the initial rfc README to detect unwanted mutation.
	rfcReadme := filepath.Join(dir, "docs", "rfc", "README.md")
	originalReadme, err := os.ReadFile(rfcReadme)
	if err != nil {
		t.Fatalf("reading rfc README after first init: %v", err)
	}

	// Drop a sentinel "user document" so we can prove it survives re-init.
	userDoc := filepath.Join(dir, "docs", "rfc", "0001-keep-me.md")
	if err := os.WriteFile(userDoc, []byte("# keep me\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Append adr to the config and re-run init.
	addition := `  adr:
    enabled: true
    dir: adr
    id_prefix: ADR
    id_width: 4
    statuses: [Accepted]
    status_field: status
`
	f, err := os.OpenFile(filepath.Join(dir, ".docz.yaml"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(addition); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	runRoot(t, "init")

	if _, err := os.Stat(filepath.Join(dir, "docs", "adr")); err != nil {
		t.Errorf("docs/adr should be scaffolded after adding adr to config: %v", err)
	}
	if _, err := os.Stat(userDoc); err != nil {
		t.Errorf("user document %s should be preserved across re-init: %v", userDoc, err)
	}
	// Without --force, the rfc README should be untouched.
	current, err := os.ReadFile(rfcReadme)
	if err != nil {
		t.Fatalf("reading rfc README after re-init: %v", err)
	}
	if !bytes.Equal(current, originalReadme) {
		t.Errorf("rfc README was mutated by re-init (no --force):\nbefore: %q\nafter:  %q",
			originalReadme, current)
	}
}
