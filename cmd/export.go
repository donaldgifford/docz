package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
	"github.com/donaldgifford/docz/v2/pkg/export/confluence"
)

// The credential variables `docz export confluence` reads. The site comes
// from sync.confluence.site; the token is never printed or logged.
const (
	envAtlassianEmail = "ATLASSIAN_EMAIL"
	envAtlassianToken = "ATLASSIAN_API_TOKEN" //nolint:gosec // the variable's name, not a credential
)

// newConfluenceClient builds the client an export talks to. A package
// variable in the runner style, so cmd tests can inject a fake and assert
// what it was asked.
var newConfluenceClient = func(site, email, token string) confluence.Client {
	return confluence.NewHTTPClient(site, email, token)
}

var (
	exportTypes  []string
	exportForce  bool
	exportDryRun bool
	exportOut    string
	exportFormat string
	exportStrict bool
)

// exportOpts is the per-invocation flag state.
type exportOpts struct {
	types  []string
	force  bool
	dryRun bool
	out    string
	format string
	strict bool
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export documents to another documentation system",
	Long: `Export documents to another documentation system.

The repository stays the source of truth: content flows out, and nothing an
export target holds is ever read back.`,
}

var exportConfluenceCmd = &cobra.Command{
	Use:   "confluence [<id>|<path>...]",
	Short: "Export documents to Confluence Cloud as pages",
	Long: `Export documents to Confluence Cloud as pages, under the parent page the
sync.confluence block of .docz.yaml names:

  sync:
    confluence:
      enabled: true
      site: https://example.atlassian.net
      space: DOCZ
      parent: docz

With no arguments every document of the configured types is exported, each
type under a page of its own carrying the type's README index. With ids or
paths only those documents are exported, and nothing is archived.

Credentials come from the environment, never from the file:

  ATLASSIAN_EMAIL       the Atlassian account's email
  ATLASSIAN_API_TOKEN   an API token; a scoped token needs
                        read:space:confluence, read:page:confluence,
                        and write:page:confluence

A page is written only when its rendered body changed. A page edited in
Confluence since docz last wrote it, or a page with the right title that
docz did not create, is skipped unless --force. On a full export, pages for
documents that no longer exist move under an Archive page; nothing is ever
deleted.

--dry-run makes every read and no write. With --out as well and no
credentials set, it renders offline: every page reports as created and its
body is written to the directory.

Exit codes:
  0  no page failed (skipped pages are warnings)
  1  a page failed, or with --strict a page was skipped
  2  configuration or usage: the block disabled, credentials missing or
     rejected, an unknown type, an id that resolves to nothing`,
	RunE: runExportConfluence,
}

func init() {
	f := exportConfluenceCmd.Flags()
	f.StringArrayVar(&exportTypes, "type", nil, "export one type (repeatable); default: sync.confluence.types")
	f.BoolVar(&exportForce, "force", false, "overwrite pages edited in Confluence and adopt pages docz did not create")
	f.BoolVar(&exportDryRun, "dry-run", false, "report what would happen; write nothing")
	f.StringVar(&exportOut, "out", "", "also write each rendered page to <dir>/<id>.xhtml")
	f.StringVar(&exportFormat, "format", formatText, "output format: text or json")
	f.BoolVar(&exportStrict, "strict", false, "exit 1 when any page was skipped")
	exportCmd.AddCommand(exportConfluenceCmd)
	rootCmd.AddCommand(exportCmd)
}

func runExportConfluence(cmd *cobra.Command, args []string) error {
	opts := exportOpts{
		types: exportTypes, force: exportForce, dryRun: exportDryRun,
		out: exportOut, format: exportFormat, strict: exportStrict,
	}

	r := getRunner()

	return r.exportConfluence(confluence.WithHooks(cmdContext(cmd), r.exportHooks()), opts, args)
}

// exportConfluence resolves flags and credentials, runs the export, prints
// the report, and returns the exit code as an error.
func (r *Runner) exportConfluence(ctx context.Context, opts exportOpts, args []string) error {
	format, err := resolveStatusFormat(opts.format)
	if err != nil {
		return err
	}

	sync := r.Cfg.Sync.Confluence
	if !sync.Enabled {
		return exitErrorf(errExitCode2, "sync.confluence is not enabled in %s", config.ConfigFileName)
	}

	client, err := r.confluenceClient(&sync, &opts)
	if err != nil {
		return err
	}

	ids, err := r.exportIDs(args)
	if err != nil {
		return err
	}

	report, runErr := confluence.Export(ctx, r.repoOrOpen(), confluence.ExportOptions{
		Client:  client,
		Types:   opts.types,
		IDs:     ids,
		Force:   opts.force,
		DryRun:  opts.dryRun,
		Resolve: blobResolver(r.Git.RemoteURL(ctx), r.Git.DefaultBranch(ctx), r.RepoRoot),
		Version: Version,
	})

	if err := r.writeExportOut(opts.out, &report); err != nil {
		return err
	}

	if len(report.Pages) > 0 || runErr == nil {
		if err := r.printExportReport(format, &report); err != nil {
			return err
		}
	}

	if runErr != nil {
		return exportExitError(runErr)
	}

	return exportOutcome(&report, opts.strict)
}

// confluenceClient builds the client, or the offline one a credential-free
// --dry-run --out uses (IMPL-0023 Open Question 4).
func (*Runner) confluenceClient(sync *config.ConfluenceSyncConfig, opts *exportOpts) (confluence.Client, error) {
	email, token := os.Getenv(envAtlassianEmail), os.Getenv(envAtlassianToken)
	if email != "" && token != "" {
		return newConfluenceClient(sync.Site, email, token), nil
	}

	if opts.dryRun && opts.out != "" {
		return offlineClient{}, nil
	}

	return nil, exitErrorf(errExitCode2,
		"%s and %s must both be set to export to %s (or use --dry-run --out <dir> to render offline)",
		envAtlassianEmail, envAtlassianToken, sync.Site)
}

// exportIDs turns arguments into document ids: an id as given, a path by
// the id in its frontmatter.
func (r *Runner) exportIDs(args []string) ([]string, error) {
	ids := make([]string, 0, len(args))

	for _, arg := range args {
		if !strings.HasSuffix(arg, ".md") {
			ids = append(ids, arg)

			continue
		}

		fm, _, err := document.LoadFrontmatter(r.inRepo(arg))
		if err != nil || fm.ID == "" {
			return nil, exitErrorf(errExitCode2, "%s: no document id in its frontmatter", arg)
		}

		ids = append(ids, fm.ID)
	}

	return ids, nil
}

// blobResolver places a link to a file that is not exported on GitHub, at
// the default branch. A remote that is not GitHub-shaped leaves every such
// link unresolved, as does one that climbs out of the repository or names
// a file the checkout under root does not have: a blob URL to nothing is a
// 404, and the report's unresolved line is the more useful thing to say.
func blobResolver(remote, branch, root string) confluence.LinkResolver {
	if remote == "" {
		return nil
	}

	return func(from, href string) confluence.LinkTarget {
		file, frag, hasFrag := strings.Cut(href, "#")

		rel := path.Clean(path.Join(path.Dir(from), file))
		if rel == ".." || strings.HasPrefix(rel, "../") || path.IsAbs(file) {
			return confluence.LinkTarget{}
		}

		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			return confluence.LinkTarget{}
		}

		url := remote + "/blob/" + branch + "/" + rel
		if hasFrag {
			url += "#" + frag
		}

		return confluence.LinkTarget{URL: url}
	}
}

// unsafeName matches what a page title may not keep in a filename.
var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// writeExportOut writes each rendered page to <dir>/<id>.xhtml; a page with
// no document id is named by its title.
func (*Runner) writeExportOut(dir string, report *confluence.Report) error {
	if dir == "" {
		return nil
	}

	if err := os.MkdirAll(dir, config.DirMode); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	for i := range report.Pages {
		p := &report.Pages[i]
		if len(p.Body) == 0 {
			continue
		}

		name := p.ID
		if name == "" {
			name = strings.Trim(unsafeName.ReplaceAllString(p.Title, "-"), "-")
		}

		file := filepath.Join(dir, name+".xhtml")
		if err := os.WriteFile(file, p.Body, config.FileMode); err != nil {
			return fmt.Errorf("writing %s: %w", file, err)
		}
	}

	return nil
}

// printExportReport prints one line per page and the totals, or the report
// as JSON.
func (r *Runner) printExportReport(format string, report *confluence.Report) error {
	if format == formatJSON {
		enc := json.NewEncoder(r.Out)
		enc.SetIndent("", "  ")

		return enc.Encode(report)
	}

	var b strings.Builder

	for i := range report.Pages {
		p := &report.Pages[i]
		fmt.Fprintf(&b, "%-10s %s%s\n", p.Action, p.Title, exportDetail(p))

		for _, l := range p.Links {
			fmt.Fprintf(&b, "  unresolved link: %s:%d -> %s\n", p.Source, l.Line, l.Href)
		}
	}

	b.WriteString(exportTotals(report) + "\n")

	_, err := io.WriteString(r.Out, b.String())

	return err
}

// exportDetail is what follows a page's title on its report line.
func exportDetail(p *confluence.PageResult) string {
	switch p.Action {
	case confluence.Created, confluence.Updated:
		parts := make([]string, 0, 2)
		if p.Version > 0 {
			parts = append(parts, fmt.Sprintf("v%d", p.Version))
		}

		if p.URL != "" {
			parts = append(parts, p.URL)
		}

		if len(parts) == 0 {
			return ""
		}

		return "  " + strings.Join(parts, "  ")
	case confluence.Skipped:
		return "  " + p.Reason + "; use --force"
	case confluence.Archived:
		return "  moved under Archive"
	case confluence.Failed:
		return "  " + p.Reason
	default:
		return ""
	}
}

// exportTotals is the summary line: "6 pages: 1 created, 2 unchanged".
func exportTotals(report *confluence.Report) string {
	var parts []string

	for _, a := range []confluence.Action{
		confluence.Created, confluence.Updated, confluence.Unchanged,
		confluence.Skipped, confluence.Archived, confluence.Failed,
	} {
		if n := report.Count(a); n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, a))
		}
	}

	line := fmt.Sprintf("%d pages", len(report.Pages))
	if len(parts) > 0 {
		line += ": " + strings.Join(parts, ", ")
	}

	if report.DryRun {
		line += " (dry run: nothing written)"
	}

	return line
}

// exportOutcome is the exit code of a run that returned no error.
func exportOutcome(report *confluence.Report, strict bool) error {
	if n := report.Count(confluence.Skipped); strict && n > 0 {
		return exitErrorf(errExitCode1, "%d pages skipped (--strict)", n)
	}

	return nil
}

// exportExitError maps Export's typed errors to exit codes: configuration
// and usage are 2, a failed page or request is 1.
func exportExitError(err error) error {
	var (
		auth     *confluence.AuthError
		notFound *repo.NotFoundError
		disabled *repo.TypeDisabledError
		unknown  *repo.UnknownTypeError
	)

	switch {
	case errors.Is(err, confluence.ErrConfig),
		errors.As(err, &auth),
		errors.As(err, &notFound),
		errors.As(err, &disabled),
		errors.As(err, &unknown),
		errors.Is(err, config.ErrUnknownType):
		return exitErrorf(errExitCode2, "%v", err)
	default:
		return exitErrorf(errExitCode1, "%v", err)
	}
}

// offlineClient is a Confluence with no pages, for a dry run that renders
// without credentials: every page reports as created and nothing is
// written, since a dry run never writes.
type offlineClient struct{}

var errOffline = errors.New("offline: no request is made on an offline dry run")

func (offlineClient) SpaceID(context.Context, string) (string, error) { return "offline", nil }

func (offlineClient) FindPage(context.Context, string, string) (*confluence.Page, error) {
	return nil, nil //nolint:nilnil // the Client contract: no page is an answer
}

func (offlineClient) CreatePage(context.Context, *confluence.NewPage) (*confluence.Page, error) {
	return nil, errOffline
}

func (offlineClient) UpdatePage(context.Context, string, *confluence.PageUpdate) (*confluence.Page, error) {
	return nil, errOffline
}

func (offlineClient) Property(context.Context, string, string) (*confluence.Property, error) {
	return nil, nil //nolint:nilnil // the Client contract: no property is an answer
}

func (offlineClient) SetProperty(context.Context, string, *confluence.Property) error {
	return errOffline
}

func (offlineClient) Children(context.Context, string) ([]confluence.Page, error) { return nil, nil }
