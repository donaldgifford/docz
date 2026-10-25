package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// fixtureRunner copies a parity fixture repository into a temp directory and
// returns a Runner over the copy. The fixtures are frozen goldens, so a test
// never reads or writes them in place.
func fixtureRunner(t *testing.T, fixture string) (*Runner, *bytes.Buffer, string) {
	t.Helper()

	root := t.TempDir()

	src := filepath.Join("..", "test", "parity", "fixtures", fixture)
	if err := os.CopyFS(root, os.DirFS(src)); err != nil {
		t.Fatalf("copying fixture %s: %v", fixture, err)
	}

	cfg, err := config.Load(filepath.Join(root, config.ConfigFileName), root)
	if err != nil {
		t.Fatalf("loading fixture config: %v", err)
	}

	var out bytes.Buffer

	return &Runner{
		Cfg:      cfg,
		Out:      &out,
		Err:      io.Discard,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:      func() time.Time { return time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC) },
		Git:      staticGit{},
		RepoRoot: root,
	}, &out, root
}

// inspectJSON runs inspect --format json and decodes the output.
func inspectJSON(t *testing.T, r *Runner, out *bytes.Buffer, arg string) inspectOutput {
	t.Helper()

	out.Reset()

	if err := r.inspect(t.Context(), formatJSON, arg); err != nil {
		t.Fatalf("inspect(%q) error = %v", arg, err)
	}

	var got inspectOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("inspect(%q) output is not JSON: %v\n%s", arg, err, out.String())
	}

	return got
}

func TestInspect_ResolvesIDAndPaths(t *testing.T) {
	r, out, root := fixtureRunner(t, "design")

	rel := filepath.Join("docs", "design", "0001-token-refresh-flow.md")

	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(body)

	for _, arg := range []string{"DESIGN-0001", rel, filepath.Join(root, rel)} {
		got := inspectJSON(t, r, out, arg)

		if got.Path != filepath.ToSlash(rel) || got.Type != "design" || got.ID != "DESIGN-0001" {
			t.Errorf("inspect(%q) = path %q type %q id %q", arg, got.Path, got.Type, got.ID)
		}

		if got.Content != string(body) || got.ContentSHA256 != hex.EncodeToString(sum[:]) {
			t.Errorf("inspect(%q) content or digest differs from the file", arg)
		}

		if got.Frontmatter == nil || got.Frontmatter.ID != "DESIGN-0001" {
			t.Errorf("inspect(%q).Frontmatter = %+v", arg, got.Frontmatter)
		}

		if got.SchemaVersion != 1 || len(got.Statuses) == 0 || len(got.Headings) == 0 {
			t.Errorf("inspect(%q) = schema_version %d, %d statuses, %d headings",
				arg, got.SchemaVersion, len(got.Statuses), len(got.Headings))
		}
	}
}

func TestInspect_RefusesWhatItDoesNotOwn(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, r *Runner, root string) string
		code  int
	}{
		{"a type name", func(*testing.T, *Runner, string) string { return "design" }, 2},
		{"an unknown id", func(*testing.T, *Runner, string) string { return "DESIGN-0099" }, 1},
		{"a missing path", func(*testing.T, *Runner, string) string { return "docs/design/0099-none.md" }, 1},
		{"a path outside the root", func(t *testing.T, _ *Runner, _ string) string {
			t.Helper()

			p := filepath.Join(t.TempDir(), "0001-x.md")
			vtestWrite(t, p, "x\n")

			return p
		}, 2},
		{"a path under a disabled type", func(t *testing.T, r *Runner, _ string) string {
			t.Helper()

			tc := r.Cfg.Types["design"]
			tc.Enabled = false
			r.Cfg.Types["design"] = tc

			return filepath.Join("docs", "design", "0001-token-refresh-flow.md")
		}, 2},
		{"a symlink escaping the root", func(t *testing.T, _ *Runner, root string) string {
			t.Helper()

			target := filepath.Join(t.TempDir(), "secret.md")
			vtestWrite(t, target, "x\n")

			link := filepath.Join(root, "docs", "design", "0009-link.md")
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}

			return link
		}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, out, root := fixtureRunner(t, "design")

			arg := tt.setup(t, r, root)

			err := r.inspect(t.Context(), formatJSON, arg)
			if err == nil {
				t.Fatalf("inspect(%q) succeeded, want exit %d\n%s", arg, tt.code, out.String())
			}

			if got := exitCodeFor(err); got != tt.code {
				t.Errorf("inspect(%q) exit = %d, want %d (err %v)", arg, got, tt.code, err)
			}

			if out.Len() != 0 {
				t.Errorf("inspect(%q) printed output on failure:\n%s", arg, out.String())
			}
		})
	}
}

func TestInspect_DuplicateIDResolvesByPath(t *testing.T) {
	r, out, root := fixtureRunner(t, "design")

	src, err := os.ReadFile(filepath.Join(root, "docs", "design", "0001-token-refresh-flow.md"))
	if err != nil {
		t.Fatal(err)
	}

	dup := filepath.Join("docs", "design", "0003-copy.md")
	vtestWrite(t, filepath.Join(root, dup), string(src))

	if got := inspectJSON(t, r, out, dup); got.Path != filepath.ToSlash(dup) {
		t.Errorf("inspect(%q).Path = %q, want the path asked for", dup, got.Path)
	}
}

func TestInspect_EveryType(t *testing.T) {
	tests := []struct{ fixture, id, typ string }{
		{"rfc", "RFC-0001", "rfc"},
		{"adr", "ADR-0001", "adr"},
		{"design", "DESIGN-0002", "design"},
		{"impl", "IMPL-0001", "impl"},
		{"investigation", "INV-0001", "investigation"},
		{"custom", "FW-0001", "frameworks"},
	}

	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			r, out, _ := fixtureRunner(t, tt.fixture)

			got := inspectJSON(t, r, out, tt.id)
			if got.Type != tt.typ || got.ID != tt.id || len(got.Regions) == 0 {
				t.Errorf("inspect(%q) = type %q id %q, %d regions", tt.id, got.Type, got.ID, len(got.Regions))
			}
		})
	}
}

func TestInspect_UnmarkedWithoutFrontmatter(t *testing.T) {
	r, out, root := fixtureRunner(t, "design")

	rel := filepath.Join("docs", "design", "draft.md")
	vtestWrite(t, filepath.Join(root, rel),
		"## Overview\n\nText.\n\n## Open Questions\n\n### 1. Which?\n\n- a. This. *(recommendation)*\n- b. That.\n")

	got := inspectJSON(t, r, out, rel)

	if got.Frontmatter != nil {
		t.Errorf("inspect().Frontmatter = %+v, want null", got.Frontmatter)
	}

	if !got.Inferred {
		t.Error("inspect().Inferred = false, want true for an unmarked document")
	}

	if len(got.Regions) == 0 || got.Regions[0].Start != 0 {
		t.Errorf("inspect().Regions = %+v, want the first starting at 0", got.Regions)
	}

	if len(got.OpenQuestions) != 1 || got.OpenQuestions[0].Resolved != nil ||
		len(got.OpenQuestions[0].Options) != 2 || !got.OpenQuestions[0].Options[0].Recommended {
		t.Errorf("inspect().OpenQuestions = %+v", got.OpenQuestions)
	}

	if !strings.Contains(out.String(), `"frontmatter": null`) || !strings.Contains(out.String(), `"resolved": null`) {
		t.Errorf("null fields are not spelled null:\n%s", out.String())
	}
}

func TestInspect_TextForm(t *testing.T) {
	r, out, _ := fixtureRunner(t, "design")

	if err := r.inspect(t.Context(), formatText, "DESIGN-0001"); err != nil {
		t.Fatalf("inspect() error = %v", err)
	}

	for _, want := range []string{"DESIGN-0001 (design)", "status:", "open questions:"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("text output lacks %q:\n%s", want, out.String())
		}
	}
}
