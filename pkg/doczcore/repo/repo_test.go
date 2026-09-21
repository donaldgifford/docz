package repo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// coreRepo builds a Repo over dir with the default configuration.
//
// Named for its file rather than generically, because several test files in
// this package each bring their own fixture builder and a shared name would
// collide.
func coreRepo(t *testing.T, dir string) *Repo {
	t.Helper()

	cfg := config.DefaultConfig()

	return &Repo{Root: dir, Cfg: &cfg}
}

// TestOpen covers what Open adds over a struct literal: it reads the config
// and it validates it. Both failures have to be distinguishable, since one
// is a missing or malformed file and the other is a file that parsed and
// says something contradictory.
func TestOpen(t *testing.T) {
	t.Parallel()

	t.Run("an empty directory yields the defaults", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		r, err := Open(t.Context(), dir, "")
		if err != nil {
			t.Fatalf("Open = %v, want nil", err)
		}

		if r.Root != dir {
			t.Errorf("Root = %q, want %q", r.Root, dir)
		}

		// No .docz.yaml is not an error: config.Load treats a missing file
		// as an empty map, so a repo with no config still has the defaults.
		if got := r.Cfg.DocsDir; got != config.DefaultConfig().DocsDir {
			t.Errorf("DocsDir = %q, want the default", got)
		}
	})

	t.Run("a repo config is merged over the defaults", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		write := filepath.Join(dir, config.ConfigFileName)

		if err := os.WriteFile(write, []byte("docs_dir: documentation\n"), config.FileMode); err != nil {
			t.Fatal(err)
		}

		r, err := Open(t.Context(), dir, "")
		if err != nil {
			t.Fatalf("Open = %v, want nil", err)
		}

		if r.Cfg.DocsDir != "documentation" {
			t.Errorf("DocsDir = %q, want %q", r.Cfg.DocsDir, "documentation")
		}

		// A sibling the file did not mention keeps its default, which is the
		// decode-onto-defaults contract config.Load keeps.
		if len(r.Cfg.Types) == 0 {
			t.Error("Types is empty; the merge dropped the defaults")
		}
	})

	t.Run("an invalid config fails at Open", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		// Two enabled types claiming the same id_prefix is a resolution
		// collision: the config parses, and Validate is what rejects it.
		// Failing here rather than at the first method call is the point of
		// Open — a consumer gets one error site, not five.
		//
		// Both types are spelled out because a types: key in a repo config
		// replaces the whole map rather than merging into it, so a fixture
		// that named only one would leave nothing for it to collide with.
		body := `types:
  rfc:
    enabled: true
    dir: rfc
    id_prefix: ADR
    id_width: 4
    statuses: [Draft]
  adr:
    enabled: true
    dir: adr
    id_prefix: ADR
    id_width: 4
    statuses: [Draft]
`
		if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName), []byte(body), config.FileMode); err != nil {
			t.Fatal(err)
		}

		if _, err := Open(t.Context(), dir, ""); err == nil {
			t.Error("Open on a colliding config = nil, want an error")
		}
	})

	t.Run("a malformed config fails at Open", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		body := "types:\n  rfc: [not, a, mapping]\n"
		if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName), []byte(body), config.FileMode); err != nil {
			t.Fatal(err)
		}

		if _, err := Open(t.Context(), dir, ""); err == nil {
			t.Error("Open on malformed yaml = nil, want an error")
		}
	})
}

// TestPathHelpers pins the three pure helpers, including the two passthrough
// cases that make a Repo built with absolute paths behave.
func TestPathHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Path", func(t *testing.T) {
		t.Parallel()

		const (
			root    = "/repo"
			outside = "/elsewhere/docs"
		)

		tests := []struct {
			name string
			root string
			rel  string
			want string
		}{
			{"joins under root", root, "docs/rfc", filepath.Join(root, "docs", "rfc")},
			{"an absolute path passes through", root, outside, outside},
			{"an empty root leaves it relative", "", "docs/rfc", "docs/rfc"},
			{"an empty path stays empty", root, "", ""},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				r := &Repo{Root: tt.root}

				if got := r.Path(tt.rel); got != tt.want {
					t.Errorf("Path(%q) = %q, want %q", tt.rel, got, tt.want)
				}
			})
		}
	})

	t.Run("TypeDir and ReadmePath", func(t *testing.T) {
		t.Parallel()

		const root = "/repo"

		r := coreRepo(t, root)

		wantDir := filepath.Join(root, "docs", "rfc")
		if got := r.TypeDir("rfc"); got != wantDir {
			t.Errorf("TypeDir(rfc) = %q, want %q", got, wantDir)
		}

		wantReadme := filepath.Join(wantDir, config.IndexFileName)
		if got := r.ReadmePath("rfc"); got != wantReadme {
			t.Errorf("ReadmePath(rfc) = %q, want %q", got, wantReadme)
		}

		// An unknown name is a path, not an error: the helper is pure and
		// total, and config.TypeDir falls back to <DocsDir>/<name>.
		wantCustom := filepath.Join(root, "docs", "frameworks")
		if got := r.TypeDir("frameworks"); got != wantCustom {
			t.Errorf("TypeDir(frameworks) = %q, want %q", got, wantCustom)
		}
	})

	t.Run("RelPath", func(t *testing.T) {
		t.Parallel()

		const root = "/repo"

		r := coreRepo(t, root)

		if got := r.RelPath(filepath.Join(root, "docs", "rfc", "0001-x.md")); got != "docs/rfc/0001-x.md" {
			t.Errorf("RelPath = %q, want %q", got, "docs/rfc/0001-x.md")
		}

		// A path outside the repository is legitimate to report; it just
		// cannot be shortened, so it comes back as it went in.
		outside := "/elsewhere/x.md"
		if got := r.RelPath(outside); !filepath.IsAbs(got) {
			t.Errorf("RelPath(%q) = %q, want an absolute path", outside, got)
		}

		empty := &Repo{}
		if got := empty.RelPath("docs/x.md"); got != "docs/x.md" {
			t.Errorf("RelPath with no root = %q, want it unchanged", got)
		}
	})
}

// TestTypesOrEnabled covers the front half every multi-type method shares:
// nil means all enabled, and an explicit token is resolved and checked.
func TestTypesOrEnabled(t *testing.T) {
	t.Parallel()

	r := coreRepo(t, t.TempDir())

	t.Run("nil means every enabled type", func(t *testing.T) {
		t.Parallel()

		got, err := r.typesOrEnabled(nil)
		if err != nil {
			t.Fatalf("typesOrEnabled(nil) = %v, want nil", err)
		}

		want := r.Cfg.EnabledTypes()
		if len(got) != len(want) {
			t.Fatalf("got %d types, want %d", len(got), len(want))
		}

		for i := range got {
			if got[i] != want[i] {
				t.Errorf("type %d = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("an empty slice means the same as nil", func(t *testing.T) {
		t.Parallel()

		got, err := r.typesOrEnabled([]string{})
		if err != nil {
			t.Fatalf("typesOrEnabled([]) = %v, want nil", err)
		}

		if len(got) != len(r.Cfg.EnabledTypes()) {
			t.Errorf("got %d types, want every enabled one", len(got))
		}
	})

	t.Run("an alias resolves to its canonical name", func(t *testing.T) {
		t.Parallel()

		got, err := r.typesOrEnabled([]string{"inv", "RFC"})
		if err != nil {
			t.Fatalf("typesOrEnabled = %v, want nil", err)
		}

		want := []string{"investigation", "rfc"}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("type %d = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("an unresolvable token is UnknownTypeError", func(t *testing.T) {
		t.Parallel()

		_, err := r.typesOrEnabled([]string{"nonsense"})

		var unknown *UnknownTypeError
		if !errors.As(err, &unknown) {
			t.Fatalf("err = %v, want *UnknownTypeError", err)
		}

		if unknown.Token != "nonsense" {
			t.Errorf("Token = %q, want the token verbatim", unknown.Token)
		}
	})

	t.Run("a disabled type asked for by name is an error", func(t *testing.T) {
		t.Parallel()

		cfg := config.DefaultConfig()
		tc := cfg.Types["rfc"]
		tc.Enabled = false
		cfg.Types["rfc"] = tc

		disabled := &Repo{Root: t.TempDir(), Cfg: &cfg}

		_, err := disabled.typesOrEnabled([]string{"rfc"})

		var want *TypeDisabledError
		if !errors.As(err, &want) {
			t.Fatalf("err = %v, want *TypeDisabledError", err)
		}

		// The same type reached through "all enabled" is simply absent,
		// so the two paths cannot disagree about it.
		all, err := disabled.typesOrEnabled(nil)
		if err != nil {
			t.Fatalf("typesOrEnabled(nil) = %v, want nil", err)
		}

		for _, name := range all {
			if name == "rfc" {
				t.Error("a disabled type appeared in the enabled set")
			}
		}
	})
}
