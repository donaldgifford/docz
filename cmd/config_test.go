package cmd

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/donaldgifford/docz/pkg/doczcore/config"
)

func TestRunConfig(t *testing.T) {
	var out bytes.Buffer
	cfg := config.DefaultConfig()
	appCfg = cfg
	runner = &Runner{
		Cfg:    cfg,
		Out:    &out,
		Err:    io.Discard,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:    time.Now,
		Git:    staticGit{},
	}
	t.Cleanup(func() { runner = nil })

	if err := runConfig(nil, nil); err != nil {
		t.Fatalf("runConfig() error: %v", err)
	}

	output := out.String()
	// Verify key config fields are present.
	if !strings.Contains(output, "docs_dir: docs") {
		t.Error("missing docs_dir in config output")
	}
	if !strings.Contains(output, "auto_update: true") {
		t.Error("missing auto_update in config output")
	}
	if !strings.Contains(output, "from_git: true") {
		t.Error("missing from_git in config output")
	}
	if !strings.Contains(output, "id_prefix: RFC") {
		t.Error("missing id_prefix for RFC in config output")
	}
	// The changelog block is dormant by default but always resolved, so
	// `docz config` is how a user discovers where docz expects it.
	if !strings.Contains(output, "changelog:") {
		t.Error("missing changelog block in config output")
	}
	if !strings.Contains(output, "file: "+config.DefaultChangelogFile) {
		t.Errorf("missing default changelog file %q in config output",
			config.DefaultChangelogFile)
	}
}

// TestRunConfig_ChangelogOverride pins that `docz config` prints the
// repo's resolved changelog block, not the defaults — the normalization
// Load applies (here the "./" strip) must be visible to the user.
func TestRunConfig_ChangelogOverride(t *testing.T) {
	var out bytes.Buffer
	cfg := config.DefaultConfig()
	cfg.Changelog = config.ChangelogConfig{
		Enabled: true,
		File:    "charts/foo/CHANGELOG.md",
	}
	appCfg = cfg
	runner = &Runner{
		Cfg:    cfg,
		Out:    &out,
		Err:    io.Discard,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:    time.Now,
		Git:    staticGit{},
	}
	t.Cleanup(func() { runner = nil })

	if err := runConfig(nil, nil); err != nil {
		t.Fatalf("runConfig() error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "enabled: true") {
		t.Error("changelog enabled override not reflected in config output")
	}
	if !strings.Contains(output, "file: charts/foo/CHANGELOG.md") {
		t.Errorf("changelog file override not reflected in config output:\n%s", output)
	}
}
