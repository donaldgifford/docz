package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/internal/config"
)

func TestRunConfig(t *testing.T) {
	appCfg = config.DefaultConfig()

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runConfig(nil, nil)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("runConfig() error: %v", err)
	}

	var buf bytes.Buffer
	if _, cpErr := buf.ReadFrom(r); cpErr != nil {
		t.Fatal(cpErr)
	}
	output := buf.String()

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
