package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/donaldgifford/docz/internal/config"
)

func TestInitSkipsDisabledTypes(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	appCfg = config.DefaultConfig()

	// Disable the "plan" and "investigation" types.
	tc := appCfg.Types["plan"]
	tc.Enabled = false
	appCfg.Types["plan"] = tc

	tc = appCfg.Types["investigation"]
	tc.Enabled = false
	appCfg.Types["investigation"] = tc

	// Suppress stdout.
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err = runInit(nil, nil)

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("runInit() error: %v", err)
	}

	// Enabled types should have directories.
	for _, typeName := range []string{"rfc", "adr", "design", "impl"} {
		typeDir := filepath.Join("docs", typeName)
		if _, err := os.Stat(typeDir); err != nil {
			t.Errorf("expected directory %s to exist for enabled type", typeDir)
		}
	}

	// Disabled types should NOT have directories.
	for _, typeName := range []string{"plan", "investigation"} {
		typeDir := filepath.Join("docs", typeName)
		if _, err := os.Stat(typeDir); err == nil {
			t.Errorf("directory %s should not exist for disabled type", typeDir)
		}
	}
}
