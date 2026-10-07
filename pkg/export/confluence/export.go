package confluence

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
)

// propertyKey is the content property that marks a page as the sync's.
const propertyKey = "docz"

// archiveTitle is the child of the parent page orphans are moved under.
const archiveTitle = "Archive"

// ExportOptions configures one Export.
type ExportOptions struct {
	// Client speaks to Confluence. Required.
	Client Client
	// Types narrows the export to these type tokens, resolved like any
	// command-line type. Nil means sync.confluence.types, or every enabled
	// type when that is empty.
	Types []string
	// IDs narrows the export to these documents. Nil means every document
	// of the selected types.
	IDs []string
	// Force overwrites a page edited in Confluence and adopts a page that
	// carries no docz property.
	Force bool
	// DryRun reads everything and writes nothing; the report says what
	// each page would have had done to it.
	DryRun bool
	// Resolve places a relative link whose target is not exported (in
	// practice, a blob URL on the forge). Links to exported files are
	// resolved by Export itself and never reach it. It is also asked for
	// each page's own source, Resolve(source, base(source)), whose URL
	// becomes the banner's link.
	Resolve LinkResolver
	// Version is the docz version written into each page's property.
	Version string
	// FS is the repository's files, rooted at the repository root and
	// addressed by slash paths: every document, README, and api: page is
	// read through it and nothing reads the disk. Nil means
	// os.DirFS(rp.Root). A server holding a fetched tree passes an
	// fstest.MapFS or its own fs.FS and needs no checkout.
	FS fs.FS
}

// Report is what an Export did.
type Report struct {
	// Pages holds one result per page in export order: the parent, then
	// each type page followed by its documents, then additional docs, then
	// orphans.
	Pages []PageResult `json:"pages"`
	// DryRun reports that no page was written; each Action is what the
	// page would have had done to it.
	DryRun bool `json:"dry_run"`
}

// Count returns how many pages took action a.
func (r *Report) Count(a Action) int {
	n := 0

	for i := range r.Pages {
		if r.Pages[i].Action == a {
			n++
		}
	}

	return n
}

// PageResult is what happened to one page.
type PageResult struct {
	// ID is the document id; empty for the parent, type, and api pages.
	ID string `json:"id,omitempty"`
	// Title is the page title.
	Title string `json:"title"`
	// Source is the repository-relative file the page was rendered from;
	// empty for a parent page with no landing page.
	Source string `json:"source,omitempty"`
	Action Action `json:"action"`
	// PageID, URL, and Version describe the page after the action; empty
	// for a page a dry run would create.
	PageID  string `json:"page_id,omitempty"`
	URL     string `json:"url,omitempty"`
	Version int    `json:"version,omitempty"`
	// Reason says why a page was Skipped or Failed.
	Reason string `json:"reason,omitempty"`
	// Links are the relative links nothing could place.
	Links []Link `json:"unresolved_links,omitempty"`
	// Body is the rendered storage format, for a caller that keeps a copy
	// (docz export confluence --out). Empty for an archived page, which is
	// moved and never rendered. Not part of the JSON report.
	Body []byte `json:"-"`
}

// Action is what an export did to one page.
type Action int

// The actions. The zero value is not an action, so a result that was never
// decided cannot pass for one.
const (
	Created Action = iota + 1
	Updated
	Unchanged
	Skipped
	Archived
	Failed
)

// String returns the action's lower-case name.
func (a Action) String() string {
	switch a {
	case Created:
		return "created"
	case Updated:
		return "updated"
	case Unchanged:
		return "unchanged"
	case Skipped:
		return "skipped"
	case Archived:
		return "archived"
	case Failed:
		return "failed"
	default:
		return fmt.Sprintf("Action(%d)", int(a))
	}
}

// MarshalText writes the action as its name, so a JSON report reads
// "created" rather than 1.
func (a Action) MarshalText() ([]byte, error) {
	return []byte(a.String()), nil
}

// Export renders and reconciles every selected document under the
// configured parent page and returns what it did (DESIGN-0020 §3).
//
// A sync.confluence block that is absent or disabled is a ConfigError
// before any request, and an unknown id is the repo.NotFoundError passed
// up. A page that fails is reported Failed and the run goes on; the error
// returned then wraps the first failure. The context is checked between
// pages, and a cancelled run returns the report so far with ctx.Err().
//
//nolint:gocritic // options by value, as DESIGN-0020 specifies and Render does
func Export(ctx context.Context, rp *repo.Repo, opts ExportOptions) (Report, error) {
	report := Report{DryRun: opts.DryRun}

	if err := checkConfig(rp, &opts); err != nil {
		return report, err
	}

	p, err := buildPlan(ctx, rp, &opts)
	if err != nil {
		return report, err
	}

	spaceID, err := opts.Client.SpaceID(ctx, rp.Cfg.Sync.Confluence.Space)
	if err != nil {
		return report, err
	}

	run := &exportRun{opts: &opts, plan: p, spaceID: spaceID, report: &report}

	if err := run.pages(ctx); err != nil {
		return report, err
	}

	if p.full {
		if err := run.orphans(ctx); err != nil {
			return report, err
		}
	}

	return report, run.firstErr
}

// checkConfig fails an export that cannot start.
func checkConfig(rp *repo.Repo, opts *ExportOptions) error {
	switch {
	case rp == nil || rp.Cfg == nil:
		return &ConfigError{Reason: "no repository configuration"}
	case !rp.Cfg.Sync.Confluence.Enabled:
		return &ConfigError{Reason: "sync.confluence is not enabled in .docz.yaml"}
	case opts.Client == nil:
		return &ConfigError{Reason: "no client"}
	}

	return nil
}

// exportRun is the state of one Export past its plan.
type exportRun struct {
	opts     *ExportOptions
	plan     *plan
	spaceID  string
	report   *Report
	firstErr error
	// pageIDs maps a plan item to its page id, once known.
	pageIDs map[int]string
	// archiveID is the Archive page, once found or created.
	archiveID string
}

// pages reconciles every plan item in order.
func (r *exportRun) pages(ctx context.Context) error {
	r.pageIDs = make(map[int]string, len(r.plan.items))

	for i := range r.plan.items {
		if err := ctx.Err(); err != nil {
			return err
		}

		it := &r.plan.items[i]

		parentID := ""
		if it.parent >= 0 {
			parentID = r.pageIDs[it.parent]
		}

		res, err := r.reconcile(ctx, it, parentID)
		r.pageIDs[i] = res.PageID
		r.record(ctx, &res, err)
	}

	return nil
}

// record appends a result, fires the hook, and keeps the first failure.
// A non-nil err makes the result Failed with err as its reason.
func (r *exportRun) record(ctx context.Context, res *PageResult, err error) {
	if err != nil {
		res.Action, res.Reason = Failed, err.Error()

		if r.firstErr == nil {
			r.firstErr = &PageError{Title: res.Title, Err: err}
		}
	}

	r.report.Pages = append(r.report.Pages, *res)
	firePageDone(ctx, res)
}

// PageError is the error Export returns when a page failed: the first one.
// Every failure is in the report. It unwraps to the cause, so an AuthError
// that failed the first page is still an AuthError to errors.As.
type PageError struct {
	Title string
	Err   error
}

func (e *PageError) Error() string {
	return fmt.Sprintf("confluence: %q failed: %v", e.Title, e.Err)
}

func (e *PageError) Unwrap() error { return e.Err }
