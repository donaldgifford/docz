package consumer

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/donaldgifford/docz/pkg/doczcore/config"
	"github.com/donaldgifford/docz/pkg/doczcore/document"
)

// doczYAML declares one built-in type (rfc) and one custom type (frameworks,
// addressable by name / id_prefix FW / alias fw) so the scan proves the
// type-agnostic contract, not just the built-ins (DESIGN-0006).
const doczYAML = `docs_dir: docs
types:
  rfc:
    enabled: true
    dir: rfc
    id_prefix: "RFC"
    id_width: 4
    statuses:
      - Draft
      - Accepted
    status_field: "status"
    plural_label: "RFCs"
  frameworks:
    enabled: true
    dir: frameworks
    id_prefix: "FW"
    id_width: 4
    statuses:
      - Draft
      - Active
    status_field: "status"
    plural_label: "Frameworks"
    aliases:
      - fw
`

const rfcDoc = `---
id: RFC-0001
title: "Adopt structured logging"
status: Accepted
author: Jane Dev
created: 2026-01-15
---

# RFC 0001: Adopt structured logging
`

const fwDoc = `---
id: FW-0001
title: "Frameworks example"
status: Active
author: Jane Dev
created: 2026-02-01
---

# FW 0001: Frameworks example
`

// TestExternalConsumerScansCustomAndBuiltinTypes runs the full ingest
// sequence an external consumer uses — Load -> Validate -> EnabledTypes ->
// ScanDocuments — over a fixture repo written to a temp dir, asserting both a
// built-in and a custom type resolve and scan correctly.
func TestExternalConsumerScansCustomAndBuiltinTypes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".docz.yaml"), doczYAML)
	writeFile(t, filepath.Join(root, "docs", "rfc", "0001-adopt-structured-logging.md"), rfcDoc)
	writeFile(t, filepath.Join(root, "docs", "frameworks", "0001-frameworks-example.md"), fwDoc)

	cfg, err := config.Load("", root)
	if err != nil {
		t.Fatalf("config.Load(\"\", %q) = %v, want nil", root, err)
	}
	if _, err := cfg.Validate(); err != nil {
		t.Fatalf("cfg.Validate() = %v, want nil", err)
	}

	enabled := cfg.EnabledTypes()
	if !slices.Contains(enabled, "rfc") {
		t.Errorf("EnabledTypes() = %v, want to contain %q", enabled, "rfc")
	}
	if !slices.Contains(enabled, "frameworks") {
		t.Errorf("EnabledTypes() = %v, want to contain %q", enabled, "frameworks")
	}

	// TypeDir is repo-relative; join it under the temp root to scan.
	fw := scanOne(t, cfg, root, "frameworks")
	if fw.ID != "FW-0001" {
		t.Errorf("frameworks scan ID = %q, want %q", fw.ID, "FW-0001")
	}
	if string(fw.Status) != "Active" {
		t.Errorf("FW-0001 status = %q, want %q", fw.Status, "Active")
	}

	rfc := scanOne(t, cfg, root, "rfc")
	if rfc.ID != "RFC-0001" {
		t.Errorf("rfc scan ID = %q, want %q", rfc.ID, "RFC-0001")
	}
}

// TestExternalConsumerParsesFrontmatterFromBytes exercises the no-checkout
// path docz-api relies on (DESIGN-0008 R3): parsing already-fetched bytes and
// the public ErrNoFrontmatter errors.Is contract.
func TestExternalConsumerParsesFrontmatterFromBytes(t *testing.T) {
	fm, err := document.ParseFrontmatter([]byte(fwDoc))
	if err != nil {
		t.Fatalf("ParseFrontmatter(fwDoc) = %v, want nil", err)
	}
	if fm.ID != "FW-0001" {
		t.Errorf("ParseFrontmatter ID = %q, want %q", fm.ID, "FW-0001")
	}
	if fm.Title != "Frameworks example" {
		t.Errorf("ParseFrontmatter Title = %q, want %q", fm.Title, "Frameworks example")
	}
	if string(fm.Status) != "Active" {
		t.Errorf("ParseFrontmatter Status = %q, want %q", fm.Status, "Active")
	}

	if _, err := document.ParseFrontmatter([]byte("no frontmatter here\n")); !errors.Is(err, document.ErrNoFrontmatter) {
		t.Errorf("ParseFrontmatter(no frontmatter) err = %v, want ErrNoFrontmatter", err)
	}
}

// scanOne scans the single document expected in cfg's type directory and
// returns it, failing the test if the count is not exactly one.
func scanOne(t *testing.T, cfg config.Config, root, docType string) document.DocEntry {
	t.Helper()
	dir := filepath.Join(root, cfg.TypeDir(docType))
	docs, err := document.ScanDocuments(dir)
	if err != nil {
		t.Fatalf("ScanDocuments(%q) = %v, want nil", dir, err)
	}
	if len(docs) != 1 {
		t.Fatalf("ScanDocuments(%q) returned %d docs, want 1", dir, len(docs))
	}
	return docs[0]
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll(%q) = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) = %v", path, err)
	}
}
