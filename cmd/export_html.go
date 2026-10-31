package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	clischema "github.com/donaldgifford/docz/v2/api/cli"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
	"github.com/donaldgifford/docz/v2/pkg/export/html"
	"github.com/donaldgifford/docz/v2/pkg/theme"
)

var (
	exportHTMLTypes   []string
	exportHTMLOut     string
	exportHTMLStdout  bool
	exportHTMLTheme   string
	exportHTMLMermaid string
	exportHTMLFormat  string
	exportHTMLDryRun  bool
)

// exportHTMLOpts is the per-invocation flag state.
type exportHTMLOpts struct {
	types   []string
	out     string
	stdout  bool
	theme   string
	mermaid string
	format  string
	dryRun  bool
}

// mermaidModes maps --mermaid to the package's modes.
var mermaidModes = map[string]html.MermaidMode{
	"embed":  html.MermaidEmbed,
	"cdn":    html.MermaidCDN,
	"source": html.MermaidSource,
}

var exportHTMLCmd = &cobra.Command{
	Use:   "html [<id>|<path>...]",
	Short: "Export documents as standalone HTML pages",
	Long: `Export documents as standalone HTML pages in docz's theme: a header with
the summary and status, a tab per section, and the theme inlined, so a page
opens from disk with no network request.

With no arguments every document of every enabled type is exported (or of
each --type), and a run of more than one page also writes index.html, a tab
per type listing each document. With ids or paths only those documents are
exported. Pages go under --out (build/html by default), mirroring each
type's directory below docs_dir. A page whose bytes on disk already match is
reported unchanged and not rewritten.

A link to a document the run exports becomes a link to its page; an image
under the repository of 1 MB or less is inlined; any other relative link
points at the file on GitHub when the repository has a GitHub remote, and is
otherwise reported unresolved.

--mermaid embed (the default) inlines mermaid in each page that has a
diagram, cdn loads it from jsDelivr under subresource integrity, and source
leaves each diagram as its source.

Exit codes:
  0  every page rendered
  1  a page failed
  2  usage or configuration: an unknown or disabled type, an unknown theme,
     mermaid mode, or format, a document that resolves to nothing, or
     --stdout with more than one page`,
	RunE: runExportHTML,
}

func init() {
	f := exportHTMLCmd.Flags()
	f.StringArrayVar(&exportHTMLTypes, "type", nil, "export one type (repeatable); default: every enabled type")
	f.StringVar(&exportHTMLOut, "out", "", "output directory (default: build/html under the repository root)")
	f.BoolVar(&exportHTMLStdout, "stdout", false, "print the one page to stdout instead of writing it")
	f.StringVar(&exportHTMLTheme, "theme", "paper", "theme: "+strings.Join(theme.Names(), ", "))
	f.StringVar(&exportHTMLMermaid, "mermaid", "embed", "mermaid diagrams: embed, cdn, or source")
	f.StringVar(&exportHTMLFormat, "format", formatText, "report format: text or json")
	f.BoolVar(&exportHTMLDryRun, "dry-run", false, "report what would be written; write nothing")
	exportCmd.AddCommand(exportHTMLCmd)
}

func runExportHTML(cmd *cobra.Command, args []string) error {
	opts := exportHTMLOpts{
		types: exportHTMLTypes, out: exportHTMLOut, stdout: exportHTMLStdout, theme: exportHTMLTheme,
		mermaid: exportHTMLMermaid, format: exportHTMLFormat, dryRun: exportHTMLDryRun,
	}

	r := getRunner()

	return r.exportHTML(html.WithHooks(cmdContext(cmd), r.exportHTMLHooks()), &opts, args)
}

// exportHTML resolves flags, runs the export, prints the report or the one
// page, and returns the exit code as an error.
func (r *Runner) exportHTML(ctx context.Context, opts *exportHTMLOpts, args []string) error {
	format, err := resolveStatusFormat(opts.format)
	if err != nil {
		return err
	}

	mode, ok := mermaidModes[strings.ToLower(opts.mermaid)]
	if !ok {
		return exitErrorf(errExitCode2, "invalid --mermaid %q (want embed, cdn, or source)", opts.mermaid)
	}

	if !slices.Contains(theme.Names(), opts.theme) {
		return exitErrorf(errExitCode2, "unknown --theme %q (want one of %s)", opts.theme, strings.Join(theme.Names(), ", "))
	}

	ids, err := r.exportIDs(args)
	if err != nil {
		return err
	}

	remote := r.Git.RemoteURL(ctx)
	blob := blobResolver(remote, r.Git.DefaultBranch(ctx), r.RepoRoot)

	report, runErr := html.Export(ctx, r.repoOrOpen(), html.ExportOptions{
		Types: opts.types, IDs: ids, Out: opts.out, Theme: opts.theme, Mermaid: mode,
		Site: siteName(remote, r.RepoRoot), Version: Version, DryRun: opts.dryRun || opts.stdout,
		Resolve: func(from, href string) string {
			if blob == nil {
				return ""
			}

			return blob(from, href).URL
		},
	})
	if runErr != nil {
		return exportHTMLExitError(runErr)
	}

	if opts.stdout {
		return r.printOnePage(&report)
	}

	if err := r.printExportHTMLReport(format, &report); err != nil {
		return err
	}

	if n := report.Failed(); n > 0 {
		return exitErrorf(errExitCode1, "%d of %d pages failed", n, len(report.Pages))
	}

	return nil
}

// siteName is the eyebrow's site: the GitHub repository's name, else the
// root directory's.
func siteName(remote, root string) string {
	if name := repositoryName(remote); name != "" {
		return path.Base(name)
	}

	return filepath.Base(root)
}

// printOnePage writes the run's one page to stdout.
func (r *Runner) printOnePage(report *html.Report) error {
	if len(report.Pages) != 1 {
		return exitErrorf(errExitCode2, "--stdout prints one page; this run has %d", len(report.Pages))
	}

	p := &report.Pages[0]
	if p.Action == html.ActionFailed {
		return exitErrorf(errExitCode1, "%s: %s", p.Source, p.Error)
	}

	if _, err := r.Out.Write(p.Body); err != nil {
		return fmt.Errorf("writing the page: %w", err)
	}

	return nil
}

// exportHTMLOutput is the JSON report: the run, its pages, and the totals.
type exportHTMLOutput struct {
	SchemaVersion int               `json:"schema_version"`
	Out           string            `json:"out"`
	DryRun        bool              `json:"dry_run"`
	Pages         []html.PageResult `json:"pages"`
	Total         exportHTMLTotal   `json:"total"`
}

type exportHTMLTotal struct {
	Pages     int `json:"pages"`
	Wrote     int `json:"wrote"`
	Unchanged int `json:"unchanged"`
	Failed    int `json:"failed"`
}

func totalOf(report *html.Report) exportHTMLTotal {
	t := exportHTMLTotal{Pages: len(report.Pages)}

	for i := range report.Pages {
		switch report.Pages[i].Action {
		case html.ActionWrote:
			t.Wrote++
		case html.ActionUnchanged:
			t.Unchanged++
		case html.ActionFailed:
			t.Failed++
		}
	}

	return t
}

// printExportHTMLReport prints one line per page and a total, or the JSON
// report. Output paths are relative to the repository root when they can
// be.
func (r *Runner) printExportHTMLReport(format string, report *html.Report) error {
	out := report.Out
	if rel, err := filepath.Rel(r.RepoRoot, out); err == nil && !strings.HasPrefix(rel, "..") {
		out = rel
	}

	if format == formatJSON {
		pages := report.Pages
		if pages == nil {
			pages = []html.PageResult{}
		}

		enc := json.NewEncoder(r.Out)
		enc.SetIndent("", "  ")

		//nolint:wrapcheck // the writer's error, as every JSON output.
		return enc.Encode(exportHTMLOutput{
			SchemaVersion: clischema.SchemaVersionExportHTML, Out: filepath.ToSlash(out), DryRun: report.DryRun,
			Pages: pages, Total: totalOf(report),
		})
	}

	verb := map[html.Action]string{html.ActionWrote: "wrote", html.ActionUnchanged: "unchanged", html.ActionFailed: "failed"}
	if report.DryRun {
		verb[html.ActionWrote] = "would write"
	}

	for i := range report.Pages {
		p := &report.Pages[i]

		name := p.ID
		if name == "" {
			name = "index"
		}

		line := fmt.Sprintf("%-11s %-14s %s", verb[p.Action], name, p.Path)

		switch {
		case p.Action == html.ActionFailed:
			line += ": " + p.Error
		case len(p.Links) > 0 || len(p.Problems) > 0:
			line += fmt.Sprintf(" (%d unresolved links, %d problems)", len(p.Links), len(p.Problems))
		}

		fmt.Fprintln(r.Out, line) //nolint:errcheck // report to stdout, as every command's.
	}

	t := totalOf(report)
	//nolint:errcheck // as above.
	fmt.Fprintf(r.Out, "%d pages in %s: %d %s, %d unchanged, %d failed\n",
		t.Pages, filepath.ToSlash(out), t.Wrote, verb[html.ActionWrote], t.Unchanged, t.Failed)

	return nil
}

// exportHTMLExitError maps a run that could not start to its exit code.
func exportHTMLExitError(err error) error {
	var (
		notFound *repo.NotFoundError
		disabled *repo.TypeDisabledError
		unknown  *repo.UnknownTypeError
	)

	switch {
	case errors.As(err, &notFound), errors.As(err, &disabled), errors.As(err, &unknown),
		errors.Is(err, config.ErrUnknownType):
		return exitErrorf(errExitCode2, "%v", err)
	default:
		return exitErrorf(errExitCode1, "%v", err)
	}
}
