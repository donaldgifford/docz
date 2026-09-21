package consumer

// The repository tier (IMPL-0018 Phase 3): pkg/doczcore/repo exercised from
// outside the module.
//
// This is the package that decides whether the v2 claim is true. Everything
// else here proves a primitive is reachable; repo proves the *operation* is,
// so a consumer that wants to scaffold a repo, refresh its indexes, move a
// status, or validate a tree does not reimplement cmd/ to get it. That every
// call below compiles and returns a typed report is the whole point.

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
)

// repoFixture builds a scaffolded repository in a temp directory and returns
// the Repo over it.
//
// Init is the setup rather than os.MkdirAll plus hand-written READMEs,
// because a consumer's first call is Init and anything it gets wrong shows
// up in every test below.
func repoFixture(t *testing.T) *repo.Repo {
	t.Helper()

	root := t.TempDir()

	r, err := repo.Open(t.Context(), root, "")
	if err != nil {
		t.Fatalf("repo.Open = %v, want nil", err)
	}

	if _, err := r.Init(t.Context(), repo.InitOptions{}); err != nil {
		t.Fatalf("Init = %v, want nil", err)
	}

	return r
}

// TestExternalConsumerOpensAndInitialisesARepo covers the two calls a
// consumer makes before it can do anything else.
func TestExternalConsumerOpensAndInitialisesARepo(t *testing.T) {
	root := t.TempDir()

	r, err := repo.Open(t.Context(), root, "")
	if err != nil {
		t.Fatalf("repo.Open = %v, want nil", err)
	}

	// No .docz.yaml is not an error. A consumer pointed at a repository that
	// has never run docz should get the defaults, not a failure.
	if r.Cfg == nil {
		t.Fatal("Open returned a Repo with no config")
	}

	report, err := r.Init(t.Context(), repo.InitOptions{})
	if err != nil {
		t.Fatalf("Init = %v, want nil", err)
	}

	if len(report.Files) == 0 {
		t.Fatal("Init reported no files")
	}

	for _, file := range report.Files {
		if file.Action != repo.InitCreated {
			t.Errorf("%s: action = %v, want InitCreated on a fresh repo", file.Path, file.Action)
		}

		// Paths are repo-relative, so a consumer can put them straight into
		// a commit or an API response without knowing the checkout location.
		if filepath.IsAbs(file.Path) {
			t.Errorf("%s is absolute; Init reports repo-relative paths", file.Path)
		}
	}

	// Running it twice is a normal thing to do and reports skips, not errors.
	second, err := r.Init(t.Context(), repo.InitOptions{})
	if err != nil {
		t.Fatalf("second Init = %v, want nil", err)
	}

	for _, file := range second.Files {
		if file.Action != repo.InitSkipped {
			t.Errorf("%s: action = %v, want InitSkipped on the second run", file.Path, file.Action)
		}
	}
}

// TestExternalConsumerCreatesScansAndFinds is the read-write round trip:
// write a document, then find it the three ways repo offers.
func TestExternalConsumerCreatesScansAndFinds(t *testing.T) {
	r := repoFixture(t)

	created, err := r.Create(t.Context(), repo.CreateOptions{
		Type:   "adr",
		Title:  "Use Postgres for the primary store",
		Author: "A Consumer",
		Update: true,
	})
	if err != nil {
		t.Fatalf("Create = %v, want nil", err)
	}

	if created.Type != "adr" {
		t.Errorf("Type = %q, want the canonical name", created.Type)
	}

	// Update: true means the index pass ran and said so, which is what lets
	// a consumer report "created and indexed" in one line.
	if created.Update == nil {
		t.Error("Create with Update: true reported no update")
	}

	docs, err := r.Scan(t.Context(), "adr")
	if err != nil {
		t.Fatalf("Scan = %v, want nil", err)
	}

	if len(docs) != 1 {
		t.Fatalf("got %d documents, want 1", len(docs))
	}

	id := docs[0].ID

	entry, err := r.Find(t.Context(), id)
	if err != nil {
		t.Fatalf("Find(%q) = %v, want nil", id, err)
	}

	// Find derives the type from the id prefix, so a consumer holding an id
	// out of a commit message never has to say which directory to look in.
	if entry.Type != "adr" {
		t.Errorf("Find picked type %q, want adr", entry.Type)
	}

	if _, err := r.FindIn(t.Context(), "adr", id); err != nil {
		t.Errorf("FindIn(%q) = %v, want nil", id, err)
	}

	entries, err := r.List(t.Context(), nil)
	if err != nil {
		t.Fatalf("List = %v, want nil", err)
	}

	if len(entries) != 1 {
		t.Errorf("List returned %d entries, want 1", len(entries))
	}

	// The typed errors are the contract a consumer branches on, so each one
	// has to be reachable from out here.
	var missing *repo.NotFoundError
	if _, err := r.FindIn(t.Context(), "adr", "ADR-9999"); !errors.As(err, &missing) {
		t.Errorf("FindIn on a missing id = %v, want *repo.NotFoundError", err)
	}

	var unknown *repo.UnknownTypeError
	if _, err := r.Scan(t.Context(), "nonsense"); !errors.As(err, &unknown) {
		t.Errorf("Scan on a bad token = %v, want *repo.UnknownTypeError", err)
	} else if !errors.Is(err, config.ErrUnknownType) {
		t.Error("the frozen v1 sentinel no longer answers errors.Is through the typed error")
	}
}

// TestExternalConsumerSetsAStatusWithDryRun is the automation path: a CI job
// that wants to know what a status change would do before doing it.
func TestExternalConsumerSetsAStatusWithDryRun(t *testing.T) {
	r := repoFixture(t)

	created, err := r.Create(t.Context(), repo.CreateOptions{Type: "adr", Title: "A decision"})
	if err != nil {
		t.Fatalf("Create = %v, want nil", err)
	}

	docs, err := r.Scan(t.Context(), "adr")
	if err != nil {
		t.Fatalf("Scan = %v, want nil", err)
	}

	id := docs[0].ID
	before, err := os.ReadFile(created.FilePath)
	if err != nil {
		t.Fatal(err)
	}

	dry, err := r.SetStatus(t.Context(), "adr", id, "Accepted", repo.StatusOptions{DryRun: true})
	if err != nil {
		t.Fatalf("SetStatus dry run = %v, want nil", err)
	}

	if dry.Changed {
		t.Error("a dry run reported Changed: true")
	}

	if dry.New != "Accepted" {
		t.Errorf("New = %q, want Accepted even on a dry run", dry.New)
	}

	after, err := os.ReadFile(created.FilePath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(before, after) {
		t.Error("a dry run wrote to the document")
	}

	live, err := r.SetStatus(t.Context(), "adr", id, "Accepted", repo.StatusOptions{})
	if err != nil {
		t.Fatalf("SetStatus = %v, want nil", err)
	}

	if !live.Changed {
		t.Error("a live run reported Changed: false")
	}

	// Setting the same value again is a no-op rather than a rewrite, which is
	// what makes the call safe to run from a loop that does not track state.
	again, err := r.SetStatus(t.Context(), "adr", id, "Accepted", repo.StatusOptions{})
	if err != nil {
		t.Fatalf("second SetStatus = %v, want nil", err)
	}

	if again.Changed {
		t.Error("setting the current status reported Changed: true")
	}

	// An invalid status carries the allowed set, so a consumer can list the
	// valid values without reaching back into the config.
	var invalid *repo.InvalidStatusError
	if _, err := r.SetStatus(t.Context(), "adr", id, "Nonsense", repo.StatusOptions{}); !errors.As(err, &invalid) {
		t.Fatalf("SetStatus with a bad status = %v, want *repo.InvalidStatusError", err)
	} else if len(invalid.Allowed) == 0 {
		t.Error("InvalidStatusError carries no Allowed set")
	}
}

// TestExternalConsumerUpdatesAndValidates covers the two reporting operations
// a CI job runs: refresh the indexes, then judge the tree.
func TestExternalConsumerUpdatesAndValidates(t *testing.T) {
	r := repoFixture(t)

	if _, err := r.Create(t.Context(), repo.CreateOptions{Type: "rfc", Title: "A proposal"}); err != nil {
		t.Fatalf("Create = %v, want nil", err)
	}

	// A dry run is what a drift check in CI wants: the report without the
	// write, so the job can fail rather than commit.
	dry, err := r.Update(t.Context(), []string{"rfc"}, repo.UpdateOptions{DryRun: true})
	if err != nil {
		t.Fatalf("Update dry run = %v, want nil", err)
	}

	if len(dry.Types) != 1 {
		t.Fatalf("got %d type reports, want 1", len(dry.Types))
	}

	if dry.Types[0].Type != "rfc" || dry.Types[0].Docs != 1 {
		t.Errorf("report = {%q, %d docs}, want {rfc, 1}", dry.Types[0].Type, dry.Types[0].Docs)
	}

	live, err := r.Update(t.Context(), nil, repo.UpdateOptions{})
	if err != nil {
		t.Fatalf("Update = %v, want nil", err)
	}

	// Nil types means every enabled type, which is what keeps a repo's
	// custom types from being silently skipped.
	if len(live.Types) != len(r.Cfg.EnabledTypes()) {
		t.Errorf("got %d type reports, want one per enabled type", len(live.Types))
	}

	report, err := r.Validate(t.Context(), []string{"rfc"}, repo.ValidateOptions{})
	if err != nil {
		t.Fatalf("Validate = %v, want nil", err)
	}

	if len(report.Templates) != 1 {
		t.Errorf("got %d template reports, want 1", len(report.Templates))
	}

	if len(report.Docs) != 1 {
		t.Fatalf("got %d document reports, want 1", len(report.Docs))
	}

	// A document docz itself created from its own template has nothing
	// structurally wrong with it, so a consumer gating on Errors > 0 does not
	// fail on a fresh repo.
	if report.Errors != 0 {
		t.Errorf("Errors = %d on a freshly created document: %+v", report.Errors, report.Docs[0].Findings)
	}

	// The schema name resolved, which is what says the document is on a
	// contract rather than getting well-formedness only.
	if report.Docs[0].Schema == "" {
		t.Error("Schema is empty; the document's schema did not resolve")
	}
}

// TestExternalConsumerMigratesADocument is the last operation: marking a
// document a consumer fetched from a repo that has never run docz.
func TestExternalConsumerMigratesADocument(t *testing.T) {
	r := repoFixture(t)

	// An unmarked document, which is every v1 document: real headings, a real
	// ToC pair, and no docz regions.
	unmarked := "---\n" +
		"id: ADR-0001\n" +
		"title: \"An older decision\"\n" +
		"status: Accepted\n" +
		"author: A Consumer\n" +
		"created: 2026-01-01\n" +
		"---\n\n" +
		"# 0001. An older decision\n\n" +
		"## Context\n\nWhy this came up.\n\n" +
		"## Decision\n\nWhat was decided.\n\n" +
		"## Consequences\n\nWhat follows.\n"

	path := filepath.Join(r.TypeDir("adr"), "0001-an-older-decision.md")
	if err := os.WriteFile(path, []byte(unmarked), config.FileMode); err != nil {
		t.Fatal(err)
	}

	// The preview first: a consumer shows the diff before writing it.
	preview, err := r.InsertRegions(t.Context(), []string{"adr"}, repo.InsertRegionsOptions{DryRun: true})
	if err != nil {
		t.Fatalf("InsertRegions dry run = %v, want nil", err)
	}

	if len(preview.Changed) != 1 {
		t.Fatalf("preview changed %d documents, want 1", len(preview.Changed))
	}

	if len(preview.Changed[0].Inserted) == 0 {
		t.Error("the preview names no kinds to insert")
	}

	if got, err := os.ReadFile(path); err != nil {
		t.Fatal(err)
	} else if string(got) != unmarked {
		t.Error("a dry run wrote to the document")
	}

	live, err := r.InsertRegions(t.Context(), []string{"adr"}, repo.InsertRegionsOptions{})
	if err != nil {
		t.Fatalf("InsertRegions = %v, want nil", err)
	}

	if len(live.Changed) != 1 {
		t.Fatalf("changed %d documents, want 1", len(live.Changed))
	}

	// Idempotent by construction: a document that carries any docz region is
	// never given more, so a consumer can run the pass on a schedule.
	repeat, err := r.InsertRegions(t.Context(), []string{"adr"}, repo.InsertRegionsOptions{})
	if err != nil {
		t.Fatalf("second InsertRegions = %v, want nil", err)
	}

	if len(repeat.Changed) != 0 {
		t.Errorf("a second pass changed %d documents, want 0", len(repeat.Changed))
	}

	if len(repeat.Unchanged) != 1 {
		t.Errorf("a second pass left %d documents unchanged, want 1", len(repeat.Unchanged))
	}
}
