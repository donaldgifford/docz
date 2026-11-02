package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// htmlOpts is the flag state of a plain docz export html.
func htmlOpts() *exportHTMLOpts {
	return &exportHTMLOpts{format: formatText, theme: "paper", mermaid: "embed"}
}

func TestExportHTMLWritesThenReportsUnchanged(t *testing.T) {
	r, out, root := fixtureRunner(t, "rfc")

	if err := r.exportHTML(t.Context(), htmlOpts(), nil); err != nil {
		t.Fatalf("exportHTML = %v", err)
	}

	if !strings.Contains(out.String(), "3 pages in build/html: 3 wrote, 0 unchanged, 0 failed") {
		t.Errorf("first run:\n%s", out)
	}

	for _, p := range []string{"index.html", "rfc/0001-switch-the-ingest-queue-to-nats.html"} {
		if _, err := os.Stat(filepath.Join(root, "build", "html", filepath.FromSlash(p))); err != nil {
			t.Errorf("%s was not written: %v", p, err)
		}
	}

	out.Reset()

	if err := r.exportHTML(t.Context(), htmlOpts(), nil); err != nil {
		t.Fatalf("exportHTML = %v", err)
	}

	if !strings.Contains(out.String(), "3 pages in build/html: 0 wrote, 3 unchanged, 0 failed") {
		t.Errorf("second run:\n%s", out)
	}
}

func TestExportHTMLDryRunWritesNothing(t *testing.T) {
	r, out, root := fixtureRunner(t, "adr")

	opts := htmlOpts()
	opts.dryRun = true

	if err := r.exportHTML(t.Context(), opts, nil); err != nil {
		t.Fatalf("exportHTML = %v", err)
	}

	if !strings.Contains(out.String(), "would write") {
		t.Errorf("a dry run does not say what it would write:\n%s", out)
	}

	if _, err := os.Stat(filepath.Join(root, "build")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a dry run wrote %s: %v", filepath.Join(root, "build"), err)
	}
}

func TestExportHTMLStdout(t *testing.T) {
	r, out, root := fixtureRunner(t, "design")

	opts := htmlOpts()
	opts.stdout = true

	if err := r.exportHTML(t.Context(), opts, []string{"DESIGN-0001"}); err != nil {
		t.Fatalf("exportHTML --stdout DESIGN-0001 = %v", err)
	}

	if !bytes.HasPrefix(out.Bytes(), []byte("<!doctype html>")) || !strings.Contains(out.String(), `data-doc="DESIGN-0001"`) {
		t.Errorf("--stdout printed:\n%.200s", out)
	}

	if _, err := os.Stat(filepath.Join(root, "build")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("--stdout wrote %s: %v", filepath.Join(root, "build"), err)
	}
}

func TestExportHTMLFragmentStdout(t *testing.T) {
	r, out, _ := fixtureRunner(t, "design")

	opts := htmlOpts()
	opts.stdout = true
	opts.fragment = true

	if err := r.exportHTML(t.Context(), opts, []string{"DESIGN-0001"}); err != nil {
		t.Fatalf("exportHTML --fragment --stdout DESIGN-0001 = %v", err)
	}

	got := out.String()
	if !strings.HasPrefix(got, `<div class="docz-page" data-theme="paper" data-doc="DESIGN-0001">`) {
		t.Errorf("--fragment printed:\n%.200s", got)
	}

	for _, banned := range []string{"<!doctype", "<style", "mode-toggle"} {
		if strings.Contains(got, banned) {
			t.Errorf("--fragment printed %q", banned)
		}
	}
}

func TestExportHTMLExitCodes(t *testing.T) {
	tests := []struct {
		name string
		edit func(*exportHTMLOpts)
		args []string
		want int
	}{
		{name: "unknown theme", edit: func(o *exportHTMLOpts) { o.theme = "neon" }, want: 2},
		{name: "unknown mermaid mode", edit: func(o *exportHTMLOpts) { o.mermaid = "png" }, want: 2},
		{name: "unknown format", edit: func(o *exportHTMLOpts) { o.format = "yaml" }, want: 2},
		{name: "unknown type", edit: func(o *exportHTMLOpts) { o.types = []string{"memo"} }, want: 2},
		{name: "disabled type", edit: func(o *exportHTMLOpts) { o.types = []string{"runbook"} }, want: 2},
		{name: "no such document", args: []string{"RFC-0099"}, want: 2},
		{name: "path with no id", args: []string{"docs/rfc/README.md"}, want: 2},
		{name: "stdout with two pages", edit: func(o *exportHTMLOpts) { o.stdout = true }, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _, _ := fixtureRunner(t, "rfc")

			opts := htmlOpts()
			if tt.edit != nil {
				tt.edit(opts)
			}

			err := r.exportHTML(t.Context(), opts, tt.args)
			if err == nil {
				t.Fatalf("exportHTML = nil, want exit %d", tt.want)
			}

			if got := exitCodeFor(err); got != tt.want {
				t.Errorf("exit %d (%v), want %d", got, err, tt.want)
			}
		})
	}
}

func TestExportHTMLFailedPageExitsOne(t *testing.T) {
	r, out, root := fixtureRunner(t, "rfc")

	path := filepath.Join(root, "docs", "rfc", "0001-switch-the-ingest-queue-to-nats.md")

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, bytes.ReplaceAll(body, []byte("\n"), []byte("\r\n")), 0o600); err != nil {
		t.Fatal(err)
	}

	err = r.exportHTML(t.Context(), htmlOpts(), nil)
	if err == nil || exitCodeFor(err) != 1 {
		t.Fatalf("exportHTML = %v, want exit 1", err)
	}

	if !strings.Contains(out.String(), "failed      RFC-0001") || !strings.Contains(out.String(), "1 failed") {
		t.Errorf("the report does not name the failed page:\n%s", out)
	}
}

// TestExportHTMLThemeFromConfig pins IMPL-0030 Phase 2: with no --theme
// the run uses html.theme, an explicit --theme wins, and the JSON report
// names the theme used.
func TestExportHTMLThemeFromConfig(t *testing.T) {
	tests := []struct {
		name, flag, want string
	}{
		{name: "the key with no flag", want: "classic"},
		{name: "the flag overrides the key", flag: "paper", want: "paper"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, out, _ := fixtureRunner(t, "adr")
			r.Cfg.HTML.Theme = "classic"

			opts := &exportHTMLOpts{format: formatJSON, theme: tt.flag, mermaid: "source", dryRun: true}
			if err := r.exportHTML(t.Context(), opts, nil); err != nil {
				t.Fatalf("exportHTML = %v", err)
			}

			if want := `"theme": "` + tt.want + `"`; !strings.Contains(out.String(), want) {
				t.Errorf("JSON report has no %s:\n%.300s", want, out)
			}
		})
	}
}
