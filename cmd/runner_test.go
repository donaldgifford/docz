package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"
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

// TestBuildLogger_VerboseSelectsDebug confirms that --verbose without
// an explicit --log-level routes Debug records to the buffer (the
// default Info handler would drop them).
func TestBuildLogger_VerboseSelectsDebug(t *testing.T) {
	var buf bytes.Buffer
	logger, err := buildLogger(&buf, true, "", "")
	if err != nil {
		t.Fatalf("buildLogger error: %v", err)
	}
	logger.Debug("hello", "k", "v")
	out := buf.String()
	if !strings.Contains(out, "hello") || !strings.Contains(out, "k=v") {
		t.Errorf("verbose=true did not emit debug record; got %q", out)
	}
}

// TestBuildLogger_DefaultDropsDebug confirms that the default level
// (no --verbose, no --log-level) is Info, so Debug records are dropped.
func TestBuildLogger_DefaultDropsDebug(t *testing.T) {
	var buf bytes.Buffer
	logger, err := buildLogger(&buf, false, "", "")
	if err != nil {
		t.Fatalf("buildLogger error: %v", err)
	}
	logger.Debug("hello")
	if got := buf.String(); got != "" {
		t.Errorf("default level should drop debug; got %q", got)
	}
}

// TestBuildLogger_LogLevelOverridesVerbose confirms that --log-level=warn
// silences debug records even when --verbose is set.
func TestBuildLogger_LogLevelOverridesVerbose(t *testing.T) {
	var buf bytes.Buffer
	logger, err := buildLogger(&buf, true, "warn", "")
	if err != nil {
		t.Fatalf("buildLogger error: %v", err)
	}
	logger.Info("info-line")
	logger.Warn("warn-line")
	out := buf.String()
	if strings.Contains(out, "info-line") {
		t.Errorf("warn level should drop info; got %q", out)
	}
	if !strings.Contains(out, "warn-line") {
		t.Errorf("warn level should keep warn; got %q", out)
	}
}

// TestBuildLogger_JSONFormat confirms that --log-format=json emits a
// JSON-decodable line per record. This pins the choice for downstream
// log aggregators that parse records by field rather than regex.
func TestBuildLogger_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	logger, err := buildLogger(&buf, false, "info", logFormatJSON)
	if err != nil {
		t.Fatalf("buildLogger error: %v", err)
	}
	logger.Info("structured", "key", "value")

	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &record); err != nil {
		t.Fatalf("json.Unmarshal failed for %q: %v", buf.String(), err)
	}
	if record["msg"] != "structured" || record["key"] != "value" {
		t.Errorf("json record missing fields: %+v", record)
	}
}

// TestBuildLogger_InvalidFlagsError pins the error path for the two
// CLI surfaces that a typo could hit.
func TestBuildLogger_InvalidFlagsError(t *testing.T) {
	if _, err := buildLogger(io.Discard, false, "trace", ""); err == nil {
		t.Error("invalid --log-level should error, got nil")
	}
	if _, err := buildLogger(io.Discard, false, "", "xml"); err == nil {
		t.Error("invalid --log-format should error, got nil")
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
