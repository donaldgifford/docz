package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/internal/config"
)

// TestPersistentPreRunE_ValidationErrorFailsCommand is the IMPL-0006 Phase 3
// regression guard: a .docz.yaml that fails Validate() must cause a
// subcommand to exit non-zero with the validation message wrapped, instead
// of silently warning and continuing on a broken config (the pre-Phase-3
// behavior in initConfig).
func TestPersistentPreRunE_ValidationErrorFailsCommand(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
		cfgFile = ""
		docsDir = ""
		appCfg = config.DefaultConfig()
	})

	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// rfc enabled but with an empty statuses list -> Validate() returns
	// "no statuses defined" error.
	content := `types:
  rfc:
    enabled: true
    dir: rfc
    statuses: []
`
	if err := os.WriteFile(".docz.yaml", []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgFile = ""
	docsDir = ""

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
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
		cfgFile = ""
		docsDir = ""
		appCfg = config.DefaultConfig()
	})

	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	content := `types:
  rfc:
    enabled: true
    dir: rfc
    statuses: []
`
	if err := os.WriteFile(".docz.yaml", []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgFile = ""
	docsDir = ""

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
