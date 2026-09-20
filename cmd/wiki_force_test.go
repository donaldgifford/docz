package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// wiki.mkdocs_path is a configuration key, so its value can arrive in a cloned
// repository, and it is not one of the keys config.Validate runs through the
// repo-relative path rules. `docz wiki init --force` spends the force by
// unlinking the file it is about to replace, which makes what that value names
// worth pinning: before the Phase 5 swap the path reached os.WriteFile and a
// directory failed there, and an unconditional unlink would instead delete it.
//
// Two tests, one file, because the guard is one condition with two sides.

// TestWikiPrepareMkDocs_LeavesADirectory is the case the guard exists for. A
// configuration naming a directory gets the pre-swap outcome: the directory
// survives and the write is what refuses it.
func TestWikiPrepareMkDocs_LeavesADirectory(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "mkdocs.yml")

	if err := os.Mkdir(target, 0o750); err != nil {
		t.Fatal(err)
	}

	if err := wikiPrepareMkDocs(target, true); err != nil {
		t.Fatalf("wikiPrepareMkDocs with --force: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("the directory was removed: %v", err)
	}

	if !info.IsDir() {
		t.Error("the directory was replaced")
	}
}

// TestWikiPrepareMkDocs_RemovesARegularFile is the case the force is for, so
// the guard cannot be satisfied by never removing anything.
func TestWikiPrepareMkDocs_RemovesARegularFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "mkdocs.yml")

	if err := os.WriteFile(target, []byte("site_name: mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := wikiPrepareMkDocs(target, true); err != nil {
		t.Fatalf("wikiPrepareMkDocs with --force: %v", err)
	}

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("the file survived --force: err = %v", err)
	}

	// And without the force it is refused rather than removed, which is the
	// other half of the function and the behaviour v1.2.2 shipped.
	if err := os.WriteFile(target, []byte("site_name: mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := wikiPrepareMkDocs(target, false); err == nil {
		t.Error("wikiPrepareMkDocs without --force accepted an existing file")
	}

	if _, err := os.Stat(target); err != nil {
		t.Errorf("the file was removed without --force: %v", err)
	}
}
