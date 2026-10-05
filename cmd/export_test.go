package cmd

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// The Confluence export through the CLI (IMPL-0023). A file of its own
// because the existing cmd/*_test.go files are frozen (ADR-0001 Decision 7).

// exportRunner builds a Runner over a temp repository with the default
// config.
func exportRunner(t *testing.T) (*Runner, *bytes.Buffer, string) {
	t.Helper()

	root := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.DocsDir = filepath.Join(root, "docs")

	var out bytes.Buffer

	return &Runner{
		Cfg:      cfg,
		Out:      &out,
		Err:      io.Discard,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:      time.Now,
		Git:      staticGit{},
		RepoRoot: root,
	}, &out, root
}

func TestExport_InitWritesTheSyncBlockDisabled(t *testing.T) {
	r, _, root := exportRunner(t)

	if err := r.Init(false); err != nil {
		t.Fatalf("init: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(root, config.ConfigFileName))
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.ParseBytes(raw)
	if err != nil {
		t.Fatalf("parsing the generated .docz.yaml: %v", err)
	}

	if !strings.Contains(string(raw), "\nsync:\n  confluence:\n    enabled: false\n") {
		t.Errorf("generated .docz.yaml has no disabled sync.confluence block:\n%s", raw)
	}

	if cfg.Sync.Confluence.Enabled || cfg.Sync.Confluence.Mermaid.Viewer != config.MermaidViewerAuto {
		t.Errorf("Sync = %+v, want disabled with viewer auto", cfg.Sync)
	}
}

func TestExport_ConfigPrintsTheSyncBlock(t *testing.T) {
	r, out, _ := exportRunner(t)
	r.Cfg.Sync.Confluence.Site = "https://example.atlassian.net"

	if err := r.Config(); err != nil {
		t.Fatalf("config: %v", err)
	}

	cfg, err := config.ParseBytes(out.Bytes())
	if err != nil {
		t.Fatalf("parsing docz config output: %v", err)
	}

	if cfg.Sync.Confluence.Site != "https://example.atlassian.net" {
		t.Errorf("round-tripped site = %q, want it printed and read back", cfg.Sync.Confluence.Site)
	}
}
