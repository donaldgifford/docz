package consumer

// The v1.0.0 surface proof (IMPL-0014 Phase 5): every pkg/doczcore
// subpackage — config, document, docparse, docwrite, toc — is imported
// by its public path and exercised the way an external consumer
// (docz-api, sdk-booty-sh) uses it. consumer_test.go covers config +
// document; this file covers the three packages v1.0.0 adds.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donaldgifford/docz/pkg/doczcore/config"
	"github.com/donaldgifford/docz/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/pkg/doczcore/docwrite"
	"github.com/donaldgifford/docz/pkg/doczcore/toc"
)

// planDoc is an IMPL-style body with headings and checkbox tasks — the
// shape an agent-loop consumer walks with docparse and writes back
// through docwrite.
const planDoc = `# IMPL 0001: Consumer Proof

<!--toc:start-->
<!--toc:end-->

## Phase 1: Setup

#### Tasks

- [x] Design the thing
- [ ] Build the thing
  - [ ] a nested subtask

## Phase 2: Ship

#### Tasks

- [ ] Ship the thing
`

// TestExternalConsumerExtractsMarkdownFacts drives docparse.Headings and
// docparse.TaskItems over fixture bytes and checks the facts an
// agent-loop consumer depends on: levels, GitHub slugs (including
// duplicate suffixing), checkbox state, indent, and 1-based lines.
func TestExternalConsumerExtractsMarkdownFacts(t *testing.T) {
	headings := docparse.Headings([]byte(planDoc))
	if len(headings) != 4 {
		t.Fatalf("Headings() returned %d headings, want 4", len(headings))
	}
	if headings[0].Level != 2 || headings[0].Slug != "phase-1-setup" || headings[0].Line != 6 {
		t.Errorf("headings[0] = %+v, want level 2 slug phase-1-setup line 6", headings[0])
	}
	if headings[3].Slug != "tasks-1" {
		t.Errorf("duplicate Tasks heading slug = %q, want %q", headings[3].Slug, "tasks-1")
	}
	if docparse.AnchorSlug("Phase 1: Setup") != "phase-1-setup" {
		t.Errorf("AnchorSlug = %q, want %q", docparse.AnchorSlug("Phase 1: Setup"), "phase-1-setup")
	}

	items := docparse.TaskItems([]byte(planDoc))
	if len(items) != 4 {
		t.Fatalf("TaskItems() returned %d items, want 4", len(items))
	}
	if !items[0].Checked || items[0].Text != "Design the thing" {
		t.Errorf("items[0] = %+v, want checked %q", items[0], "Design the thing")
	}
	if items[1].Checked || items[1].Line != 11 {
		t.Errorf("items[1] = %+v, want unchecked at line 11", items[1])
	}
	if items[2].Indent != 2 {
		t.Errorf("nested item Indent = %d, want 2", items[2].Indent)
	}
}

// TestExternalConsumerWritesBackThroughLineFacts is the docparse ->
// docwrite handshake: find an unchecked task with TaskItems, check it
// off at its reported line with CheckTask, and flip a frontmatter
// status with SetStatus — including the errors.Is sentinel contract.
func TestExternalConsumerWritesBackThroughLineFacts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "0001-consumer-proof.md")
	if err := os.WriteFile(path, []byte(fwDoc+planDoc), 0o644); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var target docparse.TaskItem
	for _, it := range docparse.TaskItems(content) {
		if !it.Checked {
			target = it
			break
		}
	}
	if target.Line == 0 {
		t.Fatal("no unchecked task found in fixture")
	}

	if err := docwrite.CheckTask(path, target.Line); err != nil {
		t.Fatalf("CheckTask(line %d) = %v, want nil", target.Line, err)
	}
	if err := docwrite.CheckTask(path, target.Line); !errors.Is(err, docwrite.ErrTaskAlreadyChecked) {
		t.Errorf("second CheckTask err = %v, want ErrTaskAlreadyChecked", err)
	}
	if err := docwrite.CheckTask(path, 100000); !errors.Is(err, docwrite.ErrLineOutOfRange) {
		t.Errorf("CheckTask(100000) err = %v, want ErrLineOutOfRange", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range docparse.TaskItems(after) {
		if it.Line == target.Line && !it.Checked {
			t.Errorf("task at line %d still unchecked after CheckTask", target.Line)
		}
	}

	oldStatus, err := docwrite.SetStatus(path, "Draft")
	if err != nil {
		t.Fatalf("SetStatus() = %v, want nil", err)
	}
	if oldStatus != "Active" {
		t.Errorf("SetStatus() old = %q, want %q", oldStatus, "Active")
	}
}

// TestExternalConsumerCreatesFromEmbeddedTemplate proves Create renders
// the embedded template through the module-private engine — the
// public->internal seam ADR-0001 Decision 3 locks in.
func TestExternalConsumerCreatesFromEmbeddedTemplate(t *testing.T) {
	docsDir := filepath.Join(t.TempDir(), "docs")

	res, err := docwrite.Create(&docwrite.CreateOptions{
		Type:      config.DocType("rfc"),
		Title:     "Consumer Proof",
		Author:    "Jane Dev",
		Status:    "Draft",
		Prefix:    "RFC",
		IDWidth:   4,
		DocsDir:   docsDir,
		TypeDir:   "rfc",
		CreatedAt: time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Create() = %v, want nil", err)
	}
	if res.Filename != "0001-consumer-proof.md" {
		t.Errorf("Filename = %q, want %q", res.Filename, "0001-consumer-proof.md")
	}

	body, err := os.ReadFile(res.FilePath)
	if err != nil {
		t.Fatalf("reading created doc: %v", err)
	}
	for _, want := range []string{"id: RFC-0001", "# RFC-0001: Consumer Proof", toc.BeginMarker} {
		if !strings.Contains(string(body), want) {
			t.Errorf("created doc missing %q", want)
		}
	}
}

// TestExternalConsumerSplicesToC runs the toc splice over marker-bearing
// content and reads the delegated docparse.Heading facts off the result.
func TestExternalConsumerSplicesToC(t *testing.T) {
	res := toc.UpdateToC(planDoc, 1)
	if !res.Found {
		t.Fatal("UpdateToC() Found = false, want true")
	}
	if !strings.Contains(res.Updated, "- [Phase 1: Setup](#phase-1-setup)") {
		t.Errorf("spliced ToC missing entry:\n%s", res.Updated)
	}
	if len(res.Headings) != 4 || res.Headings[0].Slug != "phase-1-setup" {
		t.Errorf("UpdateToC Headings = %+v, want 4 docparse headings", res.Headings)
	}

	if got := toc.UpdateToC("no markers here\n", 1); got.Found {
		t.Error("UpdateToC(no markers) Found = true, want false")
	}
}
