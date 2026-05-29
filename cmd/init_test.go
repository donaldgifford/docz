package cmd

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/donaldgifford/docz/internal/config"
)

func TestInitSkipsDisabledTypes(t *testing.T) {
	dir := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.DocsDir = filepath.Join(dir, "docs")

	// Disable the "plan" and "investigation" types.
	tc := cfg.Types["plan"]
	tc.Enabled = false
	cfg.Types["plan"] = tc

	tc = cfg.Types["investigation"]
	tc.Enabled = false
	cfg.Types["investigation"] = tc

	// Construct a Runner directly with io.Discard writers and a
	// RepoRoot pointing at the temp dir — no os.Chdir, no os.Pipe.
	runner = &Runner{
		Cfg:      cfg,
		Out:      io.Discard,
		Err:      io.Discard,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:      time.Now,
		Git:      staticGit{},
		RepoRoot: dir,
	}
	t.Cleanup(func() { runner = nil })

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("runInit() error: %v", err)
	}

	// Enabled types should have directories.
	for _, typeName := range []string{"rfc", "adr", "design", "impl"} {
		typeDir := filepath.Join(dir, "docs", typeName)
		if _, err := os.Stat(typeDir); err != nil {
			t.Errorf("expected directory %s to exist for enabled type", typeDir)
		}
	}

	// Disabled types should NOT have directories.
	for _, typeName := range []string{"plan", "investigation"} {
		typeDir := filepath.Join(dir, "docs", typeName)
		if _, err := os.Stat(typeDir); err == nil {
			t.Errorf("directory %s should not exist for disabled type", typeDir)
		}
	}

	// .docz.yaml should live in the per-test repo root, not the
	// process cwd.
	if _, err := os.Stat(filepath.Join(dir, config.ConfigFileName)); err != nil {
		t.Errorf("expected .docz.yaml at repo root: %v", err)
	}
}
