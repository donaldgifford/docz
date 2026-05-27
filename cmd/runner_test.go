package cmd

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/donaldgifford/docz/internal/config"
)

func TestNewRunner_Defaults(t *testing.T) {
	cfg := config.DefaultConfig()
	r := NewRunner(&cfg)

	if r.Out != os.Stdout {
		t.Errorf("Out = %v, want os.Stdout", r.Out)
	}
	if r.Err != os.Stderr {
		t.Errorf("Err = %v, want os.Stderr", r.Err)
	}
	if r.Logger == nil {
		t.Fatal("Logger is nil")
	}
	if r.Now == nil {
		t.Fatal("Now is nil")
	}
	if r.Git == nil {
		t.Fatal("Git is nil")
	}
	if r.Cfg.DocsDir != cfg.DocsDir {
		t.Errorf("Cfg.DocsDir = %q, want %q", r.Cfg.DocsDir, cfg.DocsDir)
	}
}

// TestRunner_DirectConstruction exercises the struct-literal pattern
// that handler tests will use after IMPL-0009 Phase 3 lands. The
// pattern is documented in DESIGN-0004 §A and §H.
func TestRunner_DirectConstruction(t *testing.T) {
	epoch := time.Unix(0, 0).UTC()
	r := &Runner{
		Cfg:    config.DefaultConfig(),
		Out:    io.Discard,
		Err:    io.Discard,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:    func() time.Time { return epoch },
		Git:    staticGit{Name: "Test User"},
	}

	if got := r.Git.UserName(context.Background()); got != "Test User" {
		t.Errorf("Git.UserName() = %q, want %q", got, "Test User")
	}
	if got := r.Now(); !got.Equal(epoch) {
		t.Errorf("Now() = %v, want %v", got, epoch)
	}
}

// TestPackageRunner_AssignedFromNewRunner confirms the package-level
// `runner` global is readable after assignment. PersistentPreRunE
// performs this assignment in production; this test guards the
// global's presence and read access without invoking Cobra.
func TestPackageRunner_AssignedFromNewRunner(t *testing.T) {
	prev := runner
	t.Cleanup(func() { runner = prev })

	cfg := config.DefaultConfig()
	runner = NewRunner(&cfg)

	if runner == nil {
		t.Fatal("runner is nil after assignment")
	}
	if runner.Out == nil {
		t.Error("runner.Out is nil")
	}
}
