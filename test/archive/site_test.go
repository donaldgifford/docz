package archive_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// oldSiteRepo is docz-site's repository before it moved into ui/ (IMPL-0020).
const oldSiteRepo = "github.com/donaldgifford/docz-site"

// siteMayCite reports whether a tracked path may still name docz-site's old
// repository: everything mayCite allows, plus the snapshot fixtures under
// ui/src/mocks/content/. Those are copies of docz-site's archived documents
// and changelog (DESIGN-0017 OQ 4), whose issue links really do point at the
// old repository, the same way the archive under docs/ does. The demo-org
// slug donaldgifford/docz-site, which the fixtures and e2e specs use as a repo
// name rather than a URL, never matches: the pattern includes github.com.
func siteMayCite(path string) bool {
	return mayCite(path) || strings.HasPrefix(path, "ui/src/mocks/content/")
}

// TestSiteRepositoryURLGone pins the URL sweep of IMPL-0020 Phase 3: nothing
// that is source, configuration, or current documentation sends a reader to
// docz-site's repository, which stopped being where the frontend lives.
func TestSiteRepositoryURLGone(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)

	cmd := exec.CommandContext(t.Context(), "git", "grep", "-l", "-F", oldSiteRepo)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		// Exit 1 with no output is git grep's "no match", which is a pass here:
		// unlike the docz-api path, nothing is required to keep citing it.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return
		}
		t.Fatalf("git grep: %v", err)
	}
	for path := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if !siteMayCite(path) {
			t.Errorf("%s names %s; point it at github.com/donaldgifford/docz", path, oldSiteRepo)
		}
	}
}

// TestUIModuleFence pins the npm-tree fence (DESIGN-0017 §3, IMPL-0020 Open
// Question 3). ui/node_modules ships stray .go files, and ui/go.mod is what
// keeps the root module's ./... from walking into them. This half catches a
// deleted ui/go.mod; the ui CI job, the one place node_modules is populated,
// catches a fence that exists but stopped working.
func TestUIModuleFence(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)

	if _, err := os.Stat(filepath.Join(root, "ui", "go.mod")); err != nil {
		t.Fatalf("ui/go.mod: %v; without it the root module reaches into ui/node_modules", err)
	}

	cmd := exec.CommandContext(t.Context(), "go", "list", "github.com/donaldgifford/docz/v2/...")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for pkg := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if strings.Contains(pkg, "/ui/") || strings.HasSuffix(pkg, "/ui") {
			t.Errorf("go list lists %s; the root module must stop at ui/", pkg)
		}
	}
}
