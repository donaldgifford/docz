package cmd

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/donaldgifford/docz/internal/config"
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
}
