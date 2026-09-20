package repo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docwrite"
)

// regionsFence is the three-backtick fence, which a Go raw string cannot
// contain.
const regionsFence = "```"

// regionsFixtureRepo builds a repository rooted in a fresh temp directory with
// the default configuration.
//
// No .docz.yaml is written: Repo's two fields are exported precisely so a
// caller that already holds a config does not have to round-trip it through
// the filesystem, and a test is that caller.
func regionsFixtureRepo(t *testing.T) *Repo {
	t.Helper()

	cfg := config.DefaultConfig()

	return &Repo{Root: t.TempDir(), Cfg: &cfg}
}

// regionsDoc assembles a document from a stock frontmatter block and the body
// lines, so a test case reads as the markdown it is about.
func regionsDoc(id string, lines ...string) string {
	head := make([]string, 0, 8+len(lines))
	head = append(head,
		"---",
		"id: "+id,
		"title: Fixture",
		"status: Draft",
		"author: Fixture",
		"created: 2026-01-01",
		"---",
		"",
	)

	return strings.Join(append(head, lines...), "\n") + "\n"
}

// regionsPutDoc writes a document into a type's directory and returns its
// absolute path.
func regionsPutDoc(t *testing.T, r *Repo, typeName, filename, body string) string {
	t.Helper()

	dir := r.TypeDir(typeName)

	if err := os.MkdirAll(dir, config.DirMode); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}

	path := filepath.Join(dir, filename)

	if err := os.WriteFile(path, []byte(body), config.FileMode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}

	return path
}

// regionsReadFile returns a file's contents as a string.
func regionsReadFile(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(b)
}

// regionsRun is the whole of a single-document case: write the body, run the
// pass over one type, and hand back the report and what is now on disk.
//
// Always a live run. The dry-run case builds its own repository, because what
// it is checking is that the two runs report the same thing while only one of
// them writes.
func regionsRun(t *testing.T, typeName, body string) (InsertRegionsReport, string) {
	t.Helper()

	r := regionsFixtureRepo(t)
	path := regionsPutDoc(t, r, typeName, "0001-fixture.md", body)

	report, err := r.InsertRegions(t.Context(), []string{typeName}, InsertRegionsOptions{})
	if err != nil {
		t.Fatalf("InsertRegions: %v", err)
	}

	return report, regionsReadFile(t, path)
}

// regionsOnlyChange returns the single Changed result, failing when the pass
// changed a different number of documents.
func regionsOnlyChange(t *testing.T, report *InsertRegionsReport) InsertRegionsResult {
	t.Helper()

	if len(report.Changed) != 1 || len(report.Unchanged) != 0 {
		t.Fatalf("want exactly one changed document, got changed=%v unchanged=%v",
			report.Changed, report.Unchanged)
	}

	return report.Changed[0]
}

// TestInsertRegionsAlreadyMarked pins the idempotence rule: a document that
// already carries a docz region is never given more, whatever its headings
// would have inferred.
//
// The rule is what makes markers authoritative. A document that names three of
// its sections is telling docz to read three, and a pass that re-inferred over
// it would mark the ones the author left out on purpose.
func TestInsertRegionsAlreadyMarked(t *testing.T) {
	t.Parallel()

	body := regionsDoc("DESIGN-0001",
		"# DESIGN-0001: Fixture",
		"",
		"<!--docz:overview:start-->",
		"## Overview",
		"",
		"Overview body.",
		"<!--docz:overview:end-->",
		"",
		"## References",
		"",
		"- [a link](https://example.com)",
	)

	report, got := regionsRun(t, "design", body)

	if len(report.Changed) != 0 {
		t.Fatalf("want no changed documents, got %v", report.Changed)
	}

	want := []string{filepath.Join("docs", "design", "0001-fixture.md")}
	if !slices.Equal(report.Unchanged, want) {
		t.Errorf("Unchanged = %v, want %v", report.Unchanged, want)
	}

	if got != body {
		t.Errorf("document was rewritten:\n%s", got)
	}
}

// TestInsertRegionsCanonicalizesSpelling pins the one edit a marked document
// does get: a marker docz read leniently is rewritten to the spelling the
// writer produces.
//
// INV-0009 Finding 4 is why leniency exists at all, and a fixer that read a
// stray space without settling it would leave the next editor to guess again.
func TestInsertRegionsCanonicalizesSpelling(t *testing.T) {
	t.Parallel()

	body := regionsDoc("DESIGN-0001",
		"# DESIGN-0001: Fixture",
		"",
		"<!-- docz:overview:start -->",
		"## Overview",
		"",
		"Overview body.",
		"<!--  docz : overview : end  -->",
	)

	want := regionsDoc("DESIGN-0001",
		"# DESIGN-0001: Fixture",
		"",
		"<!--docz:overview:start-->",
		"## Overview",
		"",
		"Overview body.",
		"<!--docz:overview:end-->",
	)

	report, got := regionsRun(t, "design", body)

	result := regionsOnlyChange(t, &report)

	if result.Fixed != 2 {
		t.Errorf("Fixed = %d, want 2", result.Fixed)
	}

	if len(result.Inserted) != 0 {
		t.Errorf("Inserted = %v, want none: a marked document is never given more", result.Inserted)
	}

	if got != want {
		t.Errorf("output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

// TestInsertRegionsInfersKinds pins the inferred kinds and the exact bytes,
// which together are the blank-line rule: a marker lands immediately above the
// span's first line and immediately below its last, so whatever blank line
// separated the section from its neighbours still separates it.
func TestInsertRegionsInfersKinds(t *testing.T) {
	t.Parallel()

	body := regionsDoc("DESIGN-0001",
		"# DESIGN-0001: Fixture",
		"",
		"## Overview",
		"",
		"Overview body.",
		"",
		"## Goals and Non-Goals",
		"",
		"### Goals",
		"",
		"- ship it",
		"",
		"### Non-Goals",
		"",
		"- rewrite it",
		"",
		"## References",
		"",
		"- [a link](https://example.com)",
	)

	want := regionsDoc("DESIGN-0001",
		"# DESIGN-0001: Fixture",
		"",
		"<!--docz:overview:start-->",
		"## Overview",
		"",
		"Overview body.",
		"<!--docz:overview:end-->",
		"",
		"## Goals and Non-Goals",
		"",
		"<!--docz:goals:start-->",
		"### Goals",
		"",
		"- ship it",
		"<!--docz:goals:end-->",
		"",
		"<!--docz:non-goals:start-->",
		"### Non-Goals",
		"",
		"- rewrite it",
		"<!--docz:non-goals:end-->",
		"",
		"<!--docz:references:start-->",
		"## References",
		"",
		"- [a link](https://example.com)",
		"<!--docz:references:end-->",
	)

	report, got := regionsRun(t, "design", body)

	result := regionsOnlyChange(t, &report)

	wantKinds := []string{"overview", "goals", "non-goals", "references"}
	if !slices.Equal(result.Inserted, wantKinds) {
		t.Errorf("Inserted = %v, want %v", result.Inserted, wantKinds)
	}

	if result.Fixed != 0 {
		t.Errorf("Fixed = %d, want 0", result.Fixed)
	}

	if got != want {
		t.Errorf("output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}

	// "Goals and Non-Goals" is a heading the design template does not wrap in
	// a region, so nothing marks it. A pass that marked every heading would
	// have invented a kind the grammar has no rule for.
	if strings.Contains(got, "goals-and-non-goals") {
		t.Error("the unmarked template heading was given a region of its own")
	}
}

// TestInsertRegionsNoHeadingMatches pins the skip rule from the other side: a
// document whose headings match nothing in the spec is left exactly as it is.
//
// docz validate then reports region.missing for each section the schema wants,
// which is the author's to fix. Inventing the heading here would mean the pass
// writing the document rather than marking it.
func TestInsertRegionsNoHeadingMatches(t *testing.T) {
	t.Parallel()

	body := regionsDoc("DESIGN-0001",
		"# DESIGN-0001: Fixture",
		"",
		"## Nothing The Spec Knows",
		"",
		"Prose.",
		"",
		"### Also Nothing",
		"",
		"More prose.",
	)

	report, got := regionsRun(t, "design", body)

	if len(report.Changed) != 0 || len(report.Unchanged) != 1 {
		t.Fatalf("want one unchanged document, got changed=%v unchanged=%v",
			report.Changed, report.Unchanged)
	}

	if got != body {
		t.Errorf("document was rewritten:\n%s", got)
	}
}

// TestInsertRegionsThematicBreakStaysOutside pins two rules at once, because
// the IMPL template is where they meet: the thematic break between phases is
// not the tail of the phase above it, and a nested pair reads as a stack —
// parents open first and close last.
func TestInsertRegionsThematicBreakStaysOutside(t *testing.T) {
	t.Parallel()

	body := regionsDoc("IMPL-0001",
		"# IMPL-0001: Fixture",
		"",
		"## Implementation Phases",
		"",
		"### Phase 1: One",
		"",
		"#### Tasks",
		"",
		"- [ ] first",
		"",
		"---",
		"",
		"### Phase 2: Two",
		"",
		"#### Tasks",
		"",
		"- [ ] second",
	)

	want := regionsDoc("IMPL-0001",
		"# IMPL-0001: Fixture",
		"",
		"## Implementation Phases",
		"",
		"<!--docz:phase:start-->",
		"### Phase 1: One",
		"",
		"<!--docz:tasks:start-->",
		"#### Tasks",
		"",
		"- [ ] first",
		"<!--docz:tasks:end-->",
		"<!--docz:phase:end-->",
		"",
		"---",
		"",
		"<!--docz:phase:start-->",
		"### Phase 2: Two",
		"",
		"<!--docz:tasks:start-->",
		"#### Tasks",
		"",
		"- [ ] second",
		"<!--docz:tasks:end-->",
		"<!--docz:phase:end-->",
	)

	report, got := regionsRun(t, "impl", body)

	result := regionsOnlyChange(t, &report)

	wantKinds := []string{"phase", "tasks", "phase", "tasks"}
	if !slices.Equal(result.Inserted, wantKinds) {
		t.Errorf("Inserted = %v, want %v", result.Inserted, wantKinds)
	}

	if got != want {
		t.Errorf("output mismatch\n got:\n%s\nwant:\n%s", got, want)
	}

	// Said again against the walker rather than against the bytes, so the
	// nesting is checked the way a reader will see it.
	regions := docparse.Regions([]byte(got))

	for i := range regions {
		if regions[i].Kind == "tasks" && regions[i].Depth != 1 {
			t.Errorf("tasks region at line %d has depth %d, want 1", regions[i].Start, regions[i].Depth)
		}

		if !regions[i].Closed {
			t.Errorf("%s region at line %d is unclosed", regions[i].Kind, regions[i].Start)
		}
	}
}

// TestInsertRegionsDryRun pins that the option changes the side effect and
// nothing else: the report is what a live run would have produced, and the
// FileWritten hook stays silent because nothing was written.
func TestInsertRegionsDryRun(t *testing.T) {
	t.Parallel()

	body := regionsDoc("DESIGN-0001",
		"# DESIGN-0001: Fixture",
		"",
		"## Overview",
		"",
		"Overview body.",
	)

	live, _ := regionsRun(t, "design", body)

	r := regionsFixtureRepo(t)
	path := regionsPutDoc(t, r, "design", "0001-fixture.md", body)

	var written []string

	ctx := WithHooks(t.Context(), &Hooks{
		FileWritten: func(p string, _ FileKind) { written = append(written, p) },
	})

	dry, err := r.InsertRegions(ctx, []string{"design"}, InsertRegionsOptions{DryRun: true})
	if err != nil {
		t.Fatalf("InsertRegions: %v", err)
	}

	if !reflect.DeepEqual(dry, live) {
		t.Errorf("dry-run report = %+v, want the live report %+v", dry, live)
	}

	if got := regionsReadFile(t, path); got != body {
		t.Errorf("dry run wrote the document:\n%s", got)
	}

	if len(written) != 0 {
		t.Errorf("FileWritten fired on a dry run for %v", written)
	}
}

// TestInsertRegionsFiresFileWritten pins the hook a real write does fire, with
// the kind that says what the file is rather than leaving a consumer to
// pattern-match the path.
//
// The path is repo-relative, like every other event the package fires. This
// site used to pass the absolute one, which made `docz validate --fix
// --verbose` log a different path shape from `docz init --verbose` for no
// reason a consumer could act on.
func TestInsertRegionsFiresFileWritten(t *testing.T) {
	t.Parallel()

	r := regionsFixtureRepo(t)
	path := regionsPutDoc(t, r, "design", "0001-fixture.md", regionsDoc("DESIGN-0001",
		"# DESIGN-0001: Fixture",
		"",
		"## Overview",
		"",
		"Overview body.",
	))

	var (
		paths []string
		found []FileKind
	)

	ctx := WithHooks(t.Context(), &Hooks{
		FileWritten: func(p string, k FileKind) {
			paths = append(paths, p)
			found = append(found, k)
		},
	})

	if _, err := r.InsertRegions(ctx, []string{"design"}, InsertRegionsOptions{}); err != nil {
		t.Fatalf("InsertRegions: %v", err)
	}

	want := []string{r.RelPath(path)}

	if !slices.Equal(paths, want) {
		t.Errorf("FileWritten paths = %v, want %v", paths, want)
	}

	if !slices.Equal(found, []FileKind{FileDocument}) {
		t.Errorf("FileWritten kinds = %v, want [%v]", found, FileDocument)
	}
}

// TestInsertRegionsCancelled pins the cancellation contract: the report
// completed so far comes back with ctx.Err(), and what was already written
// stays written.
//
// The cancel rides in on a hook, which is the only way to cancel mid-run from
// outside without a sleep. The check sits between types, so the type that was
// already in flight finishes rather than being abandoned half-marked.
func TestInsertRegionsCancelled(t *testing.T) {
	t.Parallel()

	r := regionsFixtureRepo(t)

	designPath := regionsPutDoc(t, r, "design", "0001-fixture.md", regionsDoc("DESIGN-0001",
		"# DESIGN-0001: Fixture",
		"",
		"## Overview",
		"",
		"Overview body.",
	))
	regionsPutDoc(t, r, "impl", "0001-fixture.md", regionsDoc("IMPL-0001",
		"# IMPL-0001: Fixture",
		"",
		"### Phase 1: One",
		"",
		"#### Tasks",
		"",
		"- [ ] first",
	))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ctx = WithHooks(ctx, &Hooks{
		ScanDone: func(typeName string, _ int) {
			if typeName == "design" {
				cancel()
			}
		},
	})

	report, err := r.InsertRegions(ctx, []string{"design", "impl"}, InsertRegionsOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}

	want := []string{filepath.Join("docs", "design", "0001-fixture.md")}

	got := make([]string, 0, len(report.Changed))
	for i := range report.Changed {
		got = append(got, report.Changed[i].Path)
	}

	if !slices.Equal(got, want) {
		t.Errorf("partial report Changed = %v, want %v", got, want)
	}

	if !strings.Contains(regionsReadFile(t, designPath), "<!--docz:overview:start-->") {
		t.Error("the type that finished before the cancel was rolled back")
	}
}

// TestInsertRegionsMalformedOutput pins the refusal: the pass reads its own
// output back and will not write a document whose regions say something other
// than what it inserted.
//
// The input is a document whose last section runs into an unterminated code
// fence, so the closing marker would land where docparse reads it as text. The
// region would come back unclosed and every reader would then take the rest of
// the document as part of it.
func TestInsertRegionsMalformedOutput(t *testing.T) {
	t.Parallel()

	body := regionsDoc("DESIGN-0001",
		"# DESIGN-0001: Fixture",
		"",
		"## Overview",
		"",
		regionsFence+"text",
		"the fence is never closed",
	)

	r := regionsFixtureRepo(t)
	path := regionsPutDoc(t, r, "design", "0001-fixture.md", body)

	report, err := r.InsertRegions(t.Context(), []string{"design"}, InsertRegionsOptions{})
	if !errors.Is(err, ErrMalformedOutput) {
		t.Fatalf("err = %v, want ErrMalformedOutput", err)
	}

	rel := filepath.Join("docs", "design", "0001-fixture.md")
	if !strings.Contains(err.Error(), rel) {
		t.Errorf("error %q does not name the document", err)
	}

	if !slices.Equal(report.Unchanged, []string{rel}) {
		t.Errorf("Unchanged = %v, want %v", report.Unchanged, []string{rel})
	}

	if got := regionsReadFile(t, path); got != body {
		t.Errorf("the refused document was written anyway:\n%s", got)
	}
}

// TestInsertRegionsRejectsCRLF pins that a document with CR endings is refused
// rather than spliced into.
//
// Every writer in the module rejects CR endings, and a pass that inserted
// LF-only marker lines into a CRLF document would leave a file with two kinds
// of ending and no way to tell which was the author's.
func TestInsertRegionsRejectsCRLF(t *testing.T) {
	t.Parallel()

	body := strings.ReplaceAll(regionsDoc("DESIGN-0001",
		"# DESIGN-0001: Fixture",
		"",
		"## Overview",
		"",
		"Overview body.",
	), "\n", "\r\n")

	r := regionsFixtureRepo(t)
	path := regionsPutDoc(t, r, "design", "0001-fixture.md", body)

	_, err := r.InsertRegions(t.Context(), []string{"design"}, InsertRegionsOptions{})
	if !errors.Is(err, docwrite.ErrUnsupportedLineEndings) {
		t.Fatalf("err = %v, want ErrUnsupportedLineEndings", err)
	}

	if got := regionsReadFile(t, path); got != body {
		t.Error("the refused document was written anyway")
	}
}

// TestInsertRegionsUnknownType pins that the token resolution every other
// method shares is the one this method uses too, so an alias works and a
// nonsense token is the typed error rather than an empty report.
func TestInsertRegionsUnknownType(t *testing.T) {
	t.Parallel()

	r := regionsFixtureRepo(t)

	if _, err := r.InsertRegions(t.Context(), []string{"nope"}, InsertRegionsOptions{}); err == nil {
		t.Fatal("want an error for an unresolvable type token")
	} else {
		var unknown *UnknownTypeError
		if !errors.As(err, &unknown) {
			t.Errorf("err = %v, want UnknownTypeError", err)
		}
	}
}

// TestInsertRegionsPreservesTrailingNewlines pins that the pass changes
// nothing but markers, down to how the file ends.
//
// A document ending in two newlines keeps both and one ending in none keeps
// none. Normalizing either would put a byte in the diff that no marker
// explains, which is the difference between a migration somebody reviews and a
// migration somebody has to audit.
func TestInsertRegionsPreservesTrailingNewlines(t *testing.T) {
	t.Parallel()

	base := regionsDoc("DESIGN-0001",
		"# DESIGN-0001: Fixture",
		"",
		"## Overview",
		"",
		"Overview body.",
	)

	cases := map[string]struct {
		body       string
		wantSuffix string
	}{
		"one newline":         {body: base, wantSuffix: "<!--docz:overview:end-->\n"},
		"blank line at end":   {body: base + "\n", wantSuffix: "<!--docz:overview:end-->\n\n"},
		"no trailing newline": {body: strings.TrimSuffix(base, "\n"), wantSuffix: "<!--docz:overview:end-->"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, got := regionsRun(t, "design", tc.body)

			if !strings.HasSuffix(got, tc.wantSuffix) {
				t.Errorf("output ends %q, want it to end %q", got[max(0, len(got)-40):], tc.wantSuffix)
			}
		})
	}
}
