package repo

import (
	"context"
	"testing"
)

// The hook contract has two halves worth pinning: a nil or absent Hooks is a
// no-op rather than a panic, and a hook that is set is called synchronously
// with the values the event carries. Everything else about hooks is tested
// from the operations that fire them.

func TestHooksFrom_NeverNil(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ctx  context.Context //nolint:containedctx // the table is the point.
	}{
		{name: "a bare context", ctx: context.Background()},
		{name: "an explicit nil", ctx: WithHooks(context.Background(), nil)},
		{name: "an empty Hooks", ctx: WithHooks(context.Background(), &Hooks{})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if HooksFrom(tt.ctx) == nil {
				t.Fatal("HooksFrom returned nil")
			}

			// Every fire helper over every shape: a library that panicked
			// because a consumer wired no hooks would be unusable by the
			// consumers that want none, which is most of them.
			fireScanStart(tt.ctx, "rfc", "docs/rfc")
			fireScanDone(tt.ctx, "rfc", 3)
			fireTypeSkipped(tt.ctx, "plan", SkipTypeDisabled)
			fireFileWritten(tt.ctx, "docs/rfc/README.md", FileIndex)
			fireFileSkipped(tt.ctx, "docs/rfc/README.md", SkipNoMarkers)
		})
	}
}

// TestWithHooks_ClearsInherited pins that passing nil is a way to drop hooks
// a caller inherited, rather than a no-op that leaves the parent's in place.
func TestWithHooks_ClearsInherited(t *testing.T) {
	t.Parallel()

	var called bool

	parent := WithHooks(context.Background(), &Hooks{
		ScanDone: func(string, int) { called = true },
	})

	cleared := WithHooks(parent, nil)

	fireScanDone(cleared, "rfc", 1)

	if called {
		t.Error("a hook survived WithHooks(ctx, nil)")
	}

	fireScanDone(parent, "rfc", 1)

	if !called {
		t.Error("the parent's hook did not fire")
	}
}

// TestHooks_ReceiveTheirArguments checks each event's payload once, since a
// transposed pair of string parameters is the failure a type checker cannot
// catch.
func TestHooks_ReceiveTheirArguments(t *testing.T) {
	t.Parallel()

	var (
		startType, startDir string
		doneType            string
		doneCount           int
		skippedType         string
		skippedReason       SkipReason
		writtenPath         string
		writtenKind         FileKind
		leftPath            string
		leftReason          SkipReason
	)

	ctx := WithHooks(context.Background(), &Hooks{
		ScanStart:   func(t, d string) { startType, startDir = t, d },
		ScanDone:    func(t string, n int) { doneType, doneCount = t, n },
		TypeSkipped: func(t string, r SkipReason) { skippedType, skippedReason = t, r },
		FileWritten: func(p string, k FileKind) { writtenPath, writtenKind = p, k },
		FileSkipped: func(p string, r SkipReason) { leftPath, leftReason = p, r },
	})

	fireScanStart(ctx, "adr", "docs/adr")
	fireScanDone(ctx, "adr", 7)
	fireTypeSkipped(ctx, "plan", SkipTypeDisabled)
	fireFileWritten(ctx, "docs/adr/0001-x.md", FileDocument)
	fireFileSkipped(ctx, "docs/adr/README.md", SkipNoMarkers)

	if startType != "adr" || startDir != "docs/adr" {
		t.Errorf("ScanStart got (%q, %q), want (adr, docs/adr)", startType, startDir)
	}

	if doneType != "adr" || doneCount != 7 {
		t.Errorf("ScanDone got (%q, %d), want (adr, 7)", doneType, doneCount)
	}

	if skippedType != "plan" || skippedReason != SkipTypeDisabled {
		t.Errorf("TypeSkipped got (%q, %v), want (plan, SkipTypeDisabled)", skippedType, skippedReason)
	}

	if writtenPath != "docs/adr/0001-x.md" || writtenKind != FileDocument {
		t.Errorf("FileWritten got (%q, %v), want (docs/adr/0001-x.md, FileDocument)", writtenPath, writtenKind)
	}

	if leftPath != "docs/adr/README.md" || leftReason != SkipNoMarkers {
		t.Errorf("FileSkipped got (%q, %v), want (docs/adr/README.md, SkipNoMarkers)", leftPath, leftReason)
	}
}

// TestFileKind_String and TestSkipReason_String pin one spelling per value, so
// two consumers narrating the same run cannot word it differently — and pin
// that an unrecognised value is "unknown" rather than a bare number.
func TestFileKind_String(t *testing.T) {
	t.Parallel()

	tests := map[FileKind]string{
		FileToC:      "toc",
		FileIndex:    "index",
		FileDocument: "document",
		FileConfig:   "config",
		0:            "unknown",
		FileKind(99): "unknown",
	}

	for kind, want := range tests {
		if got := kind.String(); got != want {
			t.Errorf("FileKind(%d).String() = %q, want %q", int(kind), got, want)
		}
	}
}

func TestSkipReason_String(t *testing.T) {
	t.Parallel()

	tests := map[SkipReason]string{
		SkipTypeDisabled: "type disabled",
		SkipExists:       "already exists",
		SkipNoMarkers:    "no markers",
		SkipNotDoczFile:  "not a docz file",
		0:                "unknown",
		SkipReason(99):   "unknown",
	}

	for reason, want := range tests {
		if got := reason.String(); got != want {
			t.Errorf("SkipReason(%d).String() = %q, want %q", int(reason), got, want)
		}
	}
}
