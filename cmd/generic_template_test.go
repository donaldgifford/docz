package cmd

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/doctemplate"
)

// Issue #92: a custom type declared in .docz.yaml with no template anywhere
// creates, shows, and validates from the embedded generic template, and says
// so on stderr. These tests are new rather than additions to the frozen cmd
// tests (IMPL-0025 Phase 5).

// genericTypeConfig declares one custom type and nothing to render it with.
const genericTypeConfig = `docs_dir: docs
types:
  frameworks:
    enabled: true
    dir: frameworks
    id_prefix: FW
    id_width: 4
    statuses: [Draft, Active]
`

// genericNote is the stderr line both commands print for frameworks.
const genericNote = `note: frameworks has no template; used the generic one. ` +
	`Run "docz template override frameworks" to write one.`

// setupGenericType writes genericTypeConfig to a fresh repository, runs
// `docz init` there, and returns the directory and a Runner over it whose two
// writers are returned too, so stdout and stderr can be told apart.
func setupGenericType(t *testing.T) (dir string, r *Runner, out, errOut *bytes.Buffer) {
	t.Helper()

	dir = t.TempDir()
	cfgPath := filepath.Join(dir, config.ConfigFileName)

	if err := os.WriteFile(cfgPath, []byte(genericTypeConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgFile = ""
	repoRoot = dir
	docsDir = ""

	t.Cleanup(func() {
		cfgFile = ""
		repoRoot = ""
		docsDir = ""
		appCfg = config.DefaultConfig()
		runner = nil
	})

	runRoot(t, "init")

	cfg, err := config.Load(cfgPath, dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	out, errOut = &bytes.Buffer{}, &bytes.Buffer{}
	r = &Runner{
		Cfg:      cfg,
		Out:      out,
		Err:      errOut,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:      func() time.Time { return time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC) },
		Git:      staticGit{},
		RepoRoot: dir,
	}

	return dir, r, out, errOut
}

func TestGenericTemplate_CreateNotesTheFallback(t *testing.T) {
	dir, r, out, errOut := setupGenericType(t)

	if err := r.Create(t.Context(), createOpts{}, []string{"frameworks", "A framework"}); err != nil {
		t.Fatalf("create frameworks = %v, want the generic template", err)
	}

	body, err := os.ReadFile(filepath.Join(dir, "docs", "frameworks", "0001-a-framework.md"))
	if err != nil {
		t.Fatalf("FW-0001 was not written: %v", err)
	}

	if !strings.Contains(string(body), "# FW-0001: A framework") {
		t.Errorf("FW-0001 is not the rendered generic template:\n%s", body)
	}

	if !strings.Contains(errOut.String(), genericNote) {
		t.Errorf("stderr does not carry the note:\n%s", errOut.String())
	}

	if strings.Contains(out.String(), "note:") {
		t.Errorf("the note went to stdout:\n%s", out.String())
	}

	// The document validates clean against the generic schema, under the
	// strictest threshold the CLI has.
	runRoot(t, "validate", "--strict")
}

func TestGenericTemplate_ShowPrintsTheGenericTemplate(t *testing.T) {
	_, r, out, errOut := setupGenericType(t)

	if err := r.TemplateShow(t.Context(), []string{"frameworks"}); err != nil {
		t.Fatalf("template show frameworks = %v", err)
	}

	generic, err := doctemplate.GenericTemplate()
	if err != nil {
		t.Fatalf("GenericTemplate: %v", err)
	}

	if out.String() != generic {
		t.Errorf("stdout is not exactly the generic template:\n%s", out.String())
	}

	if !strings.Contains(errOut.String(), genericNote) {
		t.Errorf("stderr does not carry the note:\n%s", errOut.String())
	}
}

func TestGenericTemplate_BuiltInPrintsNoNote(t *testing.T) {
	_, r, _, errOut := setupGenericType(t)
	r.Cfg.Types["adr"] = config.DefaultConfig().Types["adr"]

	if err := r.TemplateShow(t.Context(), []string{"adr"}); err != nil {
		t.Fatalf("template show adr = %v", err)
	}

	if errOut.Len() != 0 {
		t.Errorf("a built-in printed something on stderr:\n%s", errOut.String())
	}
}

func TestGenericTemplate_OverrideStillScaffoldsThePair(t *testing.T) {
	dir, _, _, _ := setupGenericType(t)

	runRoot(t, "template", "override", "frameworks")

	for _, rel := range []string{"templates/frameworks.md", "templates/schema/frameworks.md"} {
		if _, err := os.Stat(filepath.Join(dir, "docs", rel)); err != nil {
			t.Errorf("template override frameworks did not write docs/%s: %v", rel, err)
		}
	}
}
