package repo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/index"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/toc"
)

// updateTestRepo returns a Repo over an empty temp root whose config keeps
// only adr and rfc of the built-ins.
//
// Two types rather than six keeps a whole-repo update to two directories
// while still proving the loop iterates, and keeps the hook sequence short
// enough to write down exactly.
func updateTestRepo(t *testing.T) *Repo {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.Types = map[string]config.TypeConfig{
		"adr": cfg.Types["adr"],
		"rfc": cfg.Types["rfc"],
	}

	return &Repo{Root: t.TempDir(), Cfg: &cfg}
}

// updateTestCustomRepo is updateTestRepo plus an enabled custom type.
//
// A no-argument update has to reach it: EnabledTypes lists the built-ins
// first in registry order and appends custom keys sorted, and the bug this
// guards against is a custom type being silently skipped.
func updateTestCustomRepo(t *testing.T) *Repo {
	t.Helper()

	r := updateTestRepo(t)
	r.Cfg.Types["frameworks"] = config.TypeConfig{
		Enabled:  true,
		Dir:      "frameworks",
		IDPrefix: "FW",
		IDWidth:  4,
		Statuses: []string{"Draft"},
	}

	return r
}

// updateTestDoc writes a file into a type's directory, creating the
// directory if the case under test needs one.
func updateTestDoc(t *testing.T, r *Repo, typeName, filename, body string) {
	t.Helper()

	dir := r.TypeDir(typeName)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, filename), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// updateTestReadFile reads a file the case expects to be there.
func updateTestReadFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	return string(data)
}

// updateTestBody is a document that carries a ToC marker pair and enough
// headings to fill it — the default threshold is three — so the ToC pass
// has something to rewrite.
func updateTestBody(id string) string {
	return "---\nid: " + id + "\ntitle: \"Doc " + id + "\"\nstatus: Draft\n" +
		"author: Tester\ncreated: 2026-03-04\n---\n\n# " + id + "\n\n" +
		toc.BeginMarker + "\n" + toc.EndMarker + "\n\n" +
		"## Alpha\n\n## Beta\n\n## Gamma\n"
}

// updateTestBodyNoToC is the same document without a marker pair, which
// the ToC pass reports as skipped rather than rewriting.
func updateTestBodyNoToC(id string) string {
	return "---\nid: " + id + "\ntitle: \"Doc " + id + "\"\nstatus: Draft\n" +
		"author: Tester\ncreated: 2026-03-04\n---\n\n# " + id + "\n\n## Alpha\n"
}

// updateTestMarkedReadme is a README with the index pair already in it and
// a stale table between the markers.
func updateTestMarkedReadme() string {
	return "# Decision Records\n\nHand-written prose.\n\n" +
		index.BeginMarker + "\nstale table\n" + index.EndMarker + "\n"
}

// TestUpdate_TypeReport covers the per-type outcomes over a single type:
// what the report says, and what is on disk afterwards.
//
// The rows are data rather than closures because the interesting part of
// each case is the fixture and the expectation, not a bespoke assertion.
func TestUpdate_TypeReport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// readme is the pre-existing README body; empty writes none.
		readme string
		// docs are document bodies, written as 0001-doc.md upwards.
		docs           []string
		tocEnabled     bool
		dryRun         bool
		wantDocs       int
		wantAction     index.UpdateAction
		wantToCNil     bool
		wantToCUpdated int
		wantToCSkipped int
		wantToCWould   int
		// wantReadmeHas and wantReadmeNot are substrings of the README on
		// disk after the run; wantReadmeKept requires it to be byte-for-byte
		// what the case wrote, and wantNoReadme requires no file at all.
		wantReadmeHas  []string
		wantReadmeNot  []string
		wantReadmeKept bool
		wantNoReadme   bool
		// wantDryBodyHas are substrings of the dry-run body the report
		// carries instead of a write.
		wantDryBodyHas []string
		// wantBodyHas and wantBodyNot are substrings of the first document,
		// which is how the ToC pass is observed on disk.
		wantBodyHas []string
		wantBodyNot []string
	}{
		{
			name:          "fresh type directory",
			tocEnabled:    true,
			wantAction:    index.ActionCreated,
			wantReadmeHas: []string{index.BeginMarker, index.EndMarker, "## All ADRs"},
		},
		{
			name:           "existing marked README",
			readme:         updateTestMarkedReadme(),
			docs:           []string{updateTestBody("ADR-0001")},
			tocEnabled:     true,
			wantDocs:       1,
			wantAction:     index.ActionUpdated,
			wantToCUpdated: 1,
			wantReadmeHas:  []string{"Hand-written prose.", "ADR-0001", "## All ADRs"},
			wantReadmeNot:  []string{"stale table"},
		},
		{
			name:           "README with no markers is left alone",
			readme:         "# Mine\n\nNot docz's to rewrite.\n",
			tocEnabled:     true,
			wantAction:     index.ActionNoMarkers,
			wantReadmeKept: true,
		},
		{
			name:           "dry run over a fresh directory writes nothing",
			tocEnabled:     true,
			dryRun:         true,
			wantAction:     index.ActionDryRunCreated,
			wantNoReadme:   true,
			wantDryBodyHas: []string{index.BeginMarker, "## All ADRs"},
		},
		{
			name:           "dry run over a marked README leaves the file alone",
			readme:         updateTestMarkedReadme(),
			docs:           []string{updateTestBody("ADR-0001")},
			tocEnabled:     true,
			dryRun:         true,
			wantDocs:       1,
			wantAction:     index.ActionDryRunUpdated,
			wantToCWould:   1,
			wantReadmeKept: true,
			wantDryBodyHas: []string{"ADR-0001"},
			wantBodyNot:    []string{"- [Alpha](#alpha)"},
		},
		{
			name:        "table of contents disabled",
			docs:        []string{updateTestBody("ADR-0001")},
			wantDocs:    1,
			wantAction:  index.ActionCreated,
			wantToCNil:  true,
			wantBodyNot: []string{"- [Alpha](#alpha)"},
		},
		{
			name: "table of contents enabled counts both outcomes",
			docs: []string{
				updateTestBody("ADR-0001"),
				updateTestBodyNoToC("ADR-0002"),
			},
			tocEnabled:     true,
			wantDocs:       2,
			wantAction:     index.ActionCreated,
			wantToCUpdated: 1,
			wantToCSkipped: 1,
			wantBodyHas:    []string{"- [Alpha](#alpha)", "- [Gamma](#gamma)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := updateTestRepo(t)
			r.Cfg.TOC.Enabled = tt.tocEnabled

			if tt.readme != "" {
				updateTestDoc(t, r, "adr", config.IndexFileName, tt.readme)
			}

			for i, body := range tt.docs {
				updateTestDoc(t, r, "adr", fmt.Sprintf("%04d-doc.md", i+1), body)
			}

			report, err := r.Update(t.Context(), []string{"adr"}, UpdateOptions{DryRun: tt.dryRun})
			if err != nil {
				t.Fatalf("Update: %v", err)
			}

			if len(report.Types) != 1 {
				t.Fatalf("report holds %d types, want 1", len(report.Types))
			}

			got := &report.Types[0]

			if got.Type != "adr" {
				t.Errorf("Type = %q, want %q", got.Type, "adr")
			}

			if want := filepath.Join("docs", "adr"); got.Dir != want {
				t.Errorf("Dir = %q, want %q", got.Dir, want)
			}

			if got.Docs != tt.wantDocs {
				t.Errorf("Docs = %d, want %d", got.Docs, tt.wantDocs)
			}

			if got.Index.Action != tt.wantAction {
				t.Errorf("Index.Action = %d, want %d", got.Index.Action, tt.wantAction)
			}

			if got.Index.Path != r.ReadmePath("adr") {
				t.Errorf("Index.Path = %q, want %q", got.Index.Path, r.ReadmePath("adr"))
			}

			// A type that took no measurable time is possible on a fast
			// filesystem, so Elapsed is only checked for being set at all
			// on the path that completes.
			if got.Elapsed < 0 {
				t.Errorf("Elapsed = %v, want a non-negative duration", got.Elapsed)
			}

			if tt.wantToCNil && got.ToC != nil {
				t.Errorf("ToC = %+v, want nil with toc disabled", *got.ToC)
			}

			if !tt.wantToCNil {
				updateTestCheckToC(t, got.ToC, tt.wantToCUpdated, tt.wantToCSkipped, tt.wantToCWould)
			}

			for _, want := range tt.wantDryBodyHas {
				if !strings.Contains(got.Index.Body, want) {
					t.Errorf("dry-run body missing %q:\n%s", want, got.Index.Body)
				}
			}

			readmePath := r.ReadmePath("adr")

			if tt.wantNoReadme {
				if _, err := os.Stat(readmePath); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("stat README = %v, want it not to exist", err)
				}
			}

			var onDisk string
			if !tt.wantNoReadme {
				onDisk = updateTestReadFile(t, readmePath)
			}

			if tt.wantReadmeKept && onDisk != tt.readme {
				t.Errorf("README changed:\n%s", onDisk)
			}

			for _, want := range tt.wantReadmeHas {
				if !strings.Contains(onDisk, want) {
					t.Errorf("README missing %q:\n%s", want, onDisk)
				}
			}

			for _, not := range tt.wantReadmeNot {
				if strings.Contains(onDisk, not) {
					t.Errorf("README still holds %q:\n%s", not, onDisk)
				}
			}

			var firstDoc string
			if len(tt.docs) > 0 {
				firstDoc = updateTestReadFile(t, filepath.Join(r.TypeDir("adr"), "0001-doc.md"))
			}

			for _, want := range tt.wantBodyHas {
				if !strings.Contains(firstDoc, want) {
					t.Errorf("document missing %q:\n%s", want, firstDoc)
				}
			}

			for _, not := range tt.wantBodyNot {
				if strings.Contains(firstDoc, not) {
					t.Errorf("document holds %q it should not:\n%s", not, firstDoc)
				}
			}
		})
	}
}

// updateTestCheckToC asserts the three ToC counts a case cares about.
//
// A non-nil report with three zeros is a real expectation — the pass ran
// over a type with no documents — so the nil check is the caller's and
// this only reads the counts.
func updateTestCheckToC(t *testing.T, report *toc.UpdateReport, updated, skipped, would int) {
	t.Helper()

	if report == nil {
		t.Fatal("ToC report is nil with toc enabled")
	}

	if got := len(report.Updated); got != updated {
		t.Errorf("ToC.Updated = %d, want %d (%+v)", got, updated, report.Updated)
	}

	if got := len(report.Skipped); got != skipped {
		t.Errorf("ToC.Skipped = %d, want %d (%+v)", got, skipped, report.Skipped)
	}

	if got := len(report.WouldUpdate); got != would {
		t.Errorf("ToC.WouldUpdate = %d, want %d (%+v)", got, would, report.WouldUpdate)
	}
}

// TestUpdate_NilTypesCoversEveryEnabledType pins both the coverage and the
// order: built-ins in registry order, then custom keys sorted.
func TestUpdate_NilTypesCoversEveryEnabledType(t *testing.T) {
	t.Parallel()

	r := updateTestCustomRepo(t)

	report, err := r.Update(t.Context(), nil, UpdateOptions{})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	got := make([]string, 0, len(report.Types))
	for i := range report.Types {
		got = append(got, report.Types[i].Type)
	}

	want := []string{"rfc", "adr", "frameworks"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("types = %v, want %v", got, want)
	}

	// The custom type has no embedded index header, so it exercises the
	// generic tier and the upper-cased label that goes with it.
	custom := updateTestReadFile(t, r.ReadmePath("frameworks"))
	if !strings.Contains(custom, "All Frameworks") {
		t.Errorf("custom README missing the generated heading:\n%s", custom)
	}
}

// TestUpdate_DisabledTypeIsAnError is the one place this diverges from
// cmd/update.go, which skips a named disabled type silently.
func TestUpdate_DisabledTypeIsAnError(t *testing.T) {
	t.Parallel()

	r := updateTestRepo(t)
	r.Cfg.Types["plan"] = config.DefaultConfig().Types["plan"]

	report, err := r.Update(t.Context(), []string{"plan"}, UpdateOptions{})

	var disabled *TypeDisabledError
	if !errors.As(err, &disabled) {
		t.Fatalf("Update error = %v, want *TypeDisabledError", err)
	}

	if len(report.Types) != 0 {
		t.Errorf("report holds %d types, want none: resolution fails before any work", len(report.Types))
	}
}

// TestUpdate_UnknownTypeIsAnError pins that a bad token fails before
// anything is written, so a typo cannot half-update a repository.
func TestUpdate_UnknownTypeIsAnError(t *testing.T) {
	t.Parallel()

	r := updateTestRepo(t)

	_, err := r.Update(t.Context(), []string{"adr", "nope"}, UpdateOptions{})

	var unknown *UnknownTypeError
	if !errors.As(err, &unknown) {
		t.Fatalf("Update error = %v, want *UnknownTypeError", err)
	}

	if _, err := os.Stat(r.ReadmePath("adr")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stat adr README = %v, want nothing written", err)
	}
}

// TestUpdate_CancelledContextReturnsThePartialReport cancels during the
// first type, which is the shape the rule describes: the type in flight
// finishes, the next one never starts, and what was written stays written.
func TestUpdate_CancelledContextReturnsThePartialReport(t *testing.T) {
	t.Parallel()

	r := updateTestRepo(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ctx = WithHooks(ctx, &Hooks{
		ScanDone: func(typeName string, _ int) {
			if typeName == "rfc" {
				cancel()
			}
		},
	})

	report, err := r.Update(ctx, nil, UpdateOptions{})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Update error = %v, want context.Canceled", err)
	}

	if len(report.Types) != 1 || report.Types[0].Type != "rfc" {
		t.Fatalf("report = %+v, want just the rfc entry", report.Types)
	}

	if _, err := os.Stat(r.ReadmePath("rfc")); err != nil {
		t.Errorf("rfc README: %v, want the finished type's work kept", err)
	}

	if _, err := os.Stat(r.ReadmePath("adr")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stat adr README = %v, want the cancelled type untouched", err)
	}
}

// TestUpdate_Hooks pins the exact event sequence for a two-type update,
// which is the narration cmd/ prints and therefore the thing Phase 5 wires
// back one-to-one.
func TestUpdate_Hooks(t *testing.T) {
	t.Parallel()

	r := updateTestRepo(t)
	updateTestDoc(t, r, "rfc", "0001-first.md", updateTestBody("RFC-0001"))
	updateTestDoc(t, r, "adr", "0001-first.md", updateTestBody("ADR-0001"))

	var events []string

	ctx := WithHooks(t.Context(), &Hooks{
		ScanStart: func(typeName, dir string) {
			events = append(events, "scan-start "+typeName+" "+filepath.Base(dir))
		},
		ScanDone: func(typeName string, docs int) {
			events = append(events, fmt.Sprintf("scan-done %s %d", typeName, docs))
		},
		TypeSkipped: func(typeName string, reason SkipReason) {
			events = append(events, "type-skipped "+typeName+" "+reason.String())
		},
		FileWritten: func(path string, kind FileKind) {
			events = append(events, "written "+kind.String()+" "+filepath.Base(path))
		},
		FileSkipped: func(path string, reason SkipReason) {
			events = append(events, "skipped "+filepath.Base(path)+" "+reason.String())
		},
	})

	if _, err := r.Update(ctx, nil, UpdateOptions{}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	want := []string{
		"scan-start rfc rfc",
		"scan-done rfc 1",
		"written toc 0001-first.md",
		"written index README.md",
		"scan-start adr adr",
		"scan-done adr 1",
		"written toc 0001-first.md",
		"written index README.md",
	}

	if strings.Join(events, "\n") != strings.Join(want, "\n") {
		t.Errorf("events:\n%s\nwant:\n%s", strings.Join(events, "\n"), strings.Join(want, "\n"))
	}
}

// TestUpdate_HooksSkipUnmarkedReadme is the FileSkipped half: a README
// nobody marked is reported rather than passed over in silence, which is
// the finding INV-0009 opened.
func TestUpdate_HooksSkipUnmarkedReadme(t *testing.T) {
	t.Parallel()

	r := updateTestRepo(t)
	updateTestDoc(t, r, "adr", config.IndexFileName, "# Mine\n")

	var events []string

	ctx := WithHooks(t.Context(), &Hooks{
		FileWritten: func(path string, kind FileKind) {
			events = append(events, "written "+kind.String()+" "+filepath.Base(path))
		},
		FileSkipped: func(path string, reason SkipReason) {
			events = append(events, "skipped "+filepath.Base(path)+" "+reason.String())
		},
	})

	if _, err := r.Update(ctx, []string{"adr"}, UpdateOptions{}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if len(events) != 1 || events[0] != "skipped README.md no markers" {
		t.Errorf("events = %v, want [skipped README.md no markers]", events)
	}
}
