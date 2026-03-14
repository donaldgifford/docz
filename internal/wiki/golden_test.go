package wiki

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"
)

var update = flag.Bool("update", false, "update golden files")

func TestGoldenNavOutput(t *testing.T) {
	// Build a representative docs directory structure.
	docsDir := t.TempDir()
	navTitles := map[string]string{
		"rfc":           "RFCs",
		"adr":           "ADRs",
		"design":        "Design",
		"impl":          "Implementation Plans",
		"investigation": "Investigations",
	}
	exclude := []string{"templates", "examples"}

	// Create index.md.
	writeGoldenFile(t, docsDir, "index.md", "# My Project\n\nWelcome.\n")

	// Create rfc/ with two docs.
	mkdirAll(t, docsDir, "rfc")
	writeGoldenFile(
		t, filepath.Join(docsDir, "rfc"), "README.md",
		"# RFCs\n",
	)
	writeGoldenFile(
		t, filepath.Join(docsDir, "rfc"), "0001-api-rate-limiting.md",
		"---\nid: RFC-0001\ntitle: \"API Rate Limiting\"\nstatus: Draft\nauthor: Test\ncreated: 2026-01-01\n---\n# RFC 0001\n",
	)
	writeGoldenFile(
		t, filepath.Join(docsDir, "rfc"), "0002-event-sourcing.md",
		"---\nid: RFC-0002\ntitle: \"Event Sourcing\"\nstatus: Proposed\nauthor: Test\ncreated: 2026-01-15\n---\n# RFC 0002\n",
	)

	// Create adr/ with one doc.
	mkdirAll(t, docsDir, "adr")
	writeGoldenFile(
		t, filepath.Join(docsDir, "adr"), "README.md",
		"# ADRs\n",
	)
	writeGoldenFile(
		t, filepath.Join(docsDir, "adr"), "0001-use-postgresql.md",
		"---\nid: ADR-0001\ntitle: \"Use PostgreSQL\"\nstatus: Accepted\nauthor: Test\ncreated: 2026-02-01\n---\n# ADR 0001\n",
	)

	// Create architecture/ (non-docz).
	mkdirAll(t, docsDir, "architecture")
	writeGoldenFile(
		t, filepath.Join(docsDir, "architecture"), "system-overview.md",
		"# System Overview\n\nArchitecture details.\n",
	)
	writeGoldenFile(
		t, filepath.Join(docsDir, "architecture"), "deployment.md",
		"# Deployment\n\nDeployment guide.\n",
	)

	// Create excluded directories (should not appear).
	mkdirAll(t, docsDir, "templates")
	writeGoldenFile(
		t, filepath.Join(docsDir, "templates"), "rfc.md",
		"template content",
	)

	// Scan and sort.
	entries, err := ScanDocs(docsDir, exclude, navTitles)
	if err != nil {
		t.Fatalf("ScanDocs() error: %v", err)
	}
	entries = SortEntries(entries)

	// Serialize to YAML nav format.
	navYAML := NavToYAML(entries)
	navData := map[string]any{"nav": navYAML}
	out, err := yaml.Marshal(navData)
	if err != nil {
		t.Fatalf("yaml.Marshal() error: %v", err)
	}

	goldenPath := filepath.Join(
		"..", "..", "testdata", "golden", "wiki", "nav.yml",
	)

	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, out, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Log("Updated golden file:", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf(
			"reading golden file %s: %v\nRun with -update to create it",
			goldenPath, err,
		)
	}

	if !bytes.Equal(out, want) {
		t.Errorf(
			"nav output differs from golden file %s\nGot:\n%s\nRun with -update to update",
			goldenPath, string(out),
		)
	}
}

func writeGoldenFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
