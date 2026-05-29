package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/internal/config"
)

// writeBrokenConfig writes a `.docz.yaml` whose rfc type has an empty
// statuses list (Validate() returns "no statuses defined") into dir
// and points the package-level `repoRoot` flag at it so
// PersistentPreRunE picks dir up without any os.Chdir.
func writeBrokenConfig(t *testing.T, dir string) {
	t.Helper()
	content := `types:
  rfc:
    enabled: true
    dir: rfc
    statuses: []
`
	if err := os.WriteFile(
		filepath.Join(dir, ".docz.yaml"),
		[]byte(content),
		0o644,
	); err != nil {
		t.Fatal(err)
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
}

// TestPersistentPreRunE_ValidationErrorFailsCommand is the IMPL-0006 Phase 3
// regression guard: a .docz.yaml that fails Validate() must cause a
// subcommand to exit non-zero with the validation message wrapped, instead
// of silently warning and continuing on a broken config (the pre-Phase-3
// behavior in initConfig).
func TestPersistentPreRunE_ValidationErrorFailsCommand(t *testing.T) {
	writeBrokenConfig(t, t.TempDir())

	var stderr bytes.Buffer
	rootCmd.SetOut(&stderr)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"list", "rfc"})

	runErr := rootCmd.Execute()
	if runErr == nil {
		t.Fatal("expected validation error from PersistentPreRunE, got nil")
	}
	if !strings.Contains(runErr.Error(), "invalid config") {
		t.Errorf("error %q missing %q prefix", runErr.Error(), "invalid config")
	}
	if !strings.Contains(runErr.Error(), "no statuses") {
		t.Errorf("error %q missing wrapped Validate message", runErr.Error())
	}
}

// TestPersistentPreRunE_HelpWorksWithBrokenConfig encodes Decisions §3:
// even when .docz.yaml is broken, `docz --help` must still print help and
// exit zero. Cobra short-circuits PersistentPreRunE when the help flag is
// set; this test guards that contract so a future refactor doesn't move
// validation somewhere that runs before help is resolved.
func TestPersistentPreRunE_HelpWorksWithBrokenConfig(t *testing.T) {
	writeBrokenConfig(t, t.TempDir())

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"--help"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("docz --help with broken config returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "docz generates and manages") {
		t.Errorf("--help output missing expected long-description text: %s", buf.String())
	}
}
