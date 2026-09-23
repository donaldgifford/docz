// Package archive_test pins the import rewrite of IMPL-0019 Phase 3
// (DESIGN-0016 §5): docz-api's old module path is gone from every tracked
// source and configuration file, and still present in the archived records
// that cite it, because a record citing github.com/donaldgifford/docz-api is
// citing what was true when it was written.
package archive_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const oldModule = "github.com/donaldgifford/docz-api"

// archivedCiters are the five archived docz-api documents that name the old
// module path. A repository-wide rewrite would have changed them too.
var archivedCiters = []string{
	"docs/archive/api/design/0004-consume-the-docz-v120-api-block-pages-landing-page-and.md",
	"docs/archive/api/impl/0004-adapt-the-helm-chart-ci-and-observability-scaffolding.md",
	"docs/archive/api/impl/0008-serve-docz-yaml-key-spellings-in-config-snapshot-via-docz-v122.md",
	"docs/archive/api/impl/0009-stop-publishing-type-dir-readmes-as-directory-pages.md",
	"docs/archive/api/investigation/0006-cosign-v3-cannot-verify-our-slsa-provenance-attestations.md",
}

// mayCite reports whether a tracked path is allowed to name the old module
// path: documentation under docs/ (the records of the move itself, as well as
// the archive), snapshot fixtures under a testdata/ directory, and the
// generated CHANGELOG.md, whose entries quote commit subjects, and this test,
// which has to spell the path to look for it. Everything else is source or
// configuration, which is what the rewrite was for.
func mayCite(path string) bool {
	return strings.HasPrefix(path, "docs/") ||
		strings.HasPrefix(path, "test/archive/") ||
		strings.Contains("/"+path, "/testdata/") ||
		path == "CHANGELOG.md"
}

func repoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skipf("not in a git checkout: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func TestRewriteLeftNoOldModulePath(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)

	cmd := exec.CommandContext(t.Context(), "git", "grep", "-l", "-F", oldModule)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		// git grep exits 1 when nothing matches, which cannot happen while
		// the archive still cites the path; anything else is a real failure.
		t.Fatalf("git grep: %v", err)
	}
	for _, path := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if !mayCite(path) {
			t.Errorf("%s names %s; rewrite it to github.com/donaldgifford/docz/v2", path, oldModule)
		}
	}
}

func TestRewriteSparedTheArchive(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)

	for _, rel := range archivedCiters {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("%s: %v", rel, err)
			continue
		}
		if !bytes.Contains(body, []byte(oldModule)) {
			t.Errorf("%s no longer names %s: the archive was rewritten", rel, oldModule)
		}
	}
}
