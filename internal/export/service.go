package export

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/donaldgifford/docz/v2/internal/store"
	doczcfg "github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
	"github.com/donaldgifford/docz/v2/pkg/export/confluence"
)

var tracer = otel.Tracer("github.com/donaldgifford/docz/v2/internal/export")

// Status is a repository's export state, as confluence_syncs records it.
type Status string

// The statuses. A repository with no row is "never", which only the API
// reports.
const (
	StatusDisabled  Status = "disabled"
	StatusRefused   Status = "refused"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusPartial   Status = "partial"
	StatusFailed    Status = "failed"
)

// maxRuns bounds how often one task exports again because an ingest
// committed while it ran (DESIGN-0021 question 6).
const maxRuns = 3

// Store is the persistence the service needs. *store.Store satisfies it.
type Store interface {
	ExportInputs(ctx context.Context, repoID int64) (store.ExportInputs, error)
	UpsertConfluenceSync(ctx context.Context, p *store.UpsertConfluenceSyncParams) error
	RecordConfluenceExport(
		ctx context.Context, sync *store.UpsertConfluenceSyncParams, pages []store.UpsertConfluencePageParams,
	) error
}

var _ Store = (*store.Store)(nil)

// Allower is the server's allow-list: whether it may write to space on
// site. *config.ConfluenceConfig satisfies it.
type Allower interface {
	Allowed(site, space string) bool
}

// Counts is how many pages took each action.
type Counts struct {
	Created   int `json:"created"`
	Updated   int `json:"updated"`
	Unchanged int `json:"unchanged"`
	Skipped   int `json:"skipped"`
	Archived  int `json:"archived"`
	Failed    int `json:"failed"`
}

// countsOf tallies a report.
func countsOf(rep *confluence.Report) Counts {
	return Counts{
		Created:   rep.Count(confluence.Created),
		Updated:   rep.Count(confluence.Updated),
		Unchanged: rep.Count(confluence.Unchanged),
		Skipped:   rep.Count(confluence.Skipped),
		Archived:  rep.Count(confluence.Archived),
		Failed:    rep.Count(confluence.Failed),
	}
}

// Result is what one Run did: the last export's status and counts, and how
// many exports it took.
type Result struct {
	Repo    string
	Status  Status
	Reason  string
	Folder  string
	HeadSHA string
	Counts  Counts
	Runs    int
}

// Service runs exports. It is safe for concurrent use by the worker.
type Service struct {
	store   Store
	client  confluence.Client
	allow   Allower
	version string
	now     func() time.Time
}

// NewService builds a Service writing through client, which the process
// shares across every job, within the sites and spaces allow permits.
// version is written into each page's property.
func NewService(st Store, client confluence.Client, allow Allower, version string) *Service {
	return &Service{store: st, client: client, allow: allow, version: version, now: time.Now}
}

// Run exports one repository (DESIGN-0021 §5 steps 1 to 7). A disabled
// block or a refused site, space, or layout is recorded and is not an
// error. Any other failure is recorded and returned, wrapping
// asynq.SkipRetry when retrying cannot help. When an ingest committed a new
// head while the export ran, Run exports again, at most maxRuns times in
// all.
func (s *Service) Run(ctx context.Context, repoID int64) (Result, error) {
	var (
		res      Result
		exported string
	)

	for run := 1; run <= maxRuns; run++ {
		in, err := s.load(ctx, repoID)
		if err != nil {
			return res, err
		}

		head := in.Repo.LastSyncedSha.String
		if run > 1 && head == exported {
			return res, nil
		}

		res, err = s.once(ctx, &in)
		res.Runs = run

		if err != nil || (res.Status != StatusSucceeded && res.Status != StatusPartial) {
			return res, err
		}

		exported = head
	}

	return res, nil
}

// load reads the repository's snapshot. A repository that is gone was
// uninstalled after the job was enqueued, which no retry fixes.
func (s *Service) load(ctx context.Context, repoID int64) (store.ExportInputs, error) {
	ctx, span := tracer.Start(ctx, "export.load")
	defer span.End()

	in, err := s.store.ExportInputs(ctx, repoID)
	if err != nil {
		spanError(span, err)

		if errors.Is(err, pgx.ErrNoRows) {
			return in, fmt.Errorf("%w: repo %d is gone: %w", asynq.SkipRetry, repoID, err)
		}

		return in, fmt.Errorf("load repo %d: %w", repoID, err)
	}

	return in, nil
}

// once runs one export over a snapshot and records it.
func (s *Service) once(ctx context.Context, in *store.ExportInputs) (Result, error) {
	label := in.Repo.Owner + "/" + in.Repo.Name
	res := Result{Repo: label, HeadSHA: in.Repo.LastSyncedSha.String}

	var cfg doczcfg.Config
	if err := json.Unmarshal(in.Repo.ConfigSnapshot, &cfg); err != nil {
		res.Status, res.Reason = StatusFailed, "unreadable config snapshot"
		s.recordState(ctx, in, nil, &res)

		return res, fmt.Errorf("%w: config snapshot of %s: %w", asynq.SkipRetry, label, err)
	}

	sc := &cfg.Sync.Confluence

	switch {
	case !sc.Enabled:
		res.Status = StatusDisabled
		s.recordState(ctx, in, sc, &res)

		return res, nil
	case sc.Layout == doczcfg.LayoutPage:
		res.Status, res.Reason = StatusRefused, "layout: page is for the CLI; the server writes the folder layout"
	case !s.allow.Allowed(sc.Site, sc.Space):
		res.Status, res.Reason = StatusRefused, fmt.Sprintf("space %s on %s is not allowed on this server", sc.Space, sc.Site)
	}

	if res.Status == StatusRefused {
		slog.WarnContext(ctx, "confluence export refused", "repo", label, "reason", res.Reason)
		s.recordState(ctx, in, sc, &res)

		return res, nil
	}

	started := s.now()
	res.Status = StatusRunning
	s.recordState(ctx, in, sc, &res)

	rep, err := s.export(ctx, in, &cfg)
	status, skip := classify(err)

	res.Status, res.Counts = status, countsOf(&rep)
	if rep.Folder != nil {
		res.Folder = rep.Folder.Title
	}

	if err != nil {
		res.Reason = err.Error()
	}

	warnings(ctx, label, &rep)

	if rerr := s.record(ctx, in, sc, &res, &rep, started); rerr != nil {
		return res, errors.Join(err, rerr)
	}

	switch {
	case err == nil:
		return res, nil
	case skip:
		return res, fmt.Errorf("%w: export %s: %w", asynq.SkipRetry, label, err)
	default:
		return res, fmt.Errorf("export %s: %w", label, err)
	}
}

// export runs confluence.Export over the snapshot's files.
func (s *Service) export(ctx context.Context, in *store.ExportInputs, cfg *doczcfg.Config) (confluence.Report, error) {
	ctx, span := tracer.Start(ctx, "export.run")
	defer span.End()

	fsys, err := files(in, cfg)
	if err != nil {
		spanError(span, err)

		return confluence.Report{}, err
	}

	opts := confluence.ExportOptions{
		Client:     s.client,
		FS:         fsys,
		Repository: in.Repo.Owner + "/" + in.Repo.Name,
		Overwrite:  true,
		Pages:      in.PageIDs,
		Resolve:    blobResolver(in.Repo.Owner, in.Repo.Name, in.Repo.DefaultBranch, missingDocument(fsys, cfg)),
		Version:    s.version,
	}
	if in.Sync != nil {
		opts.Folder = in.Sync.FolderID
	}

	ctx = confluence.WithHooks(ctx, &confluence.Hooks{
		Request: func(method, path string, status int) {
			span.AddEvent("confluence.request", trace.WithAttributes(
				attribute.String("http.method", method),
				attribute.String("url.path", path),
				attribute.Int("http.status_code", status),
			))
		},
	})

	// The root is never read: every file comes through FS. It names a
	// directory that does not exist so a slip would fail, not read the
	// server's disk.
	rp := &repo.Repo{Root: "/nonexistent/docz-api/export", Cfg: cfg}

	rep, err := confluence.Export(ctx, rp, opts)
	if err != nil {
		spanError(span, err)
	}

	return rep, err
}

// warnings logs each page another repository owns and each inline comment
// an update could not carry (DESIGN-0021 §9).
func warnings(ctx context.Context, label string, rep *confluence.Report) {
	for i := range rep.Pages {
		r := &rep.Pages[i]

		if r.Action == confluence.Skipped && strings.HasPrefix(r.Reason, confluence.ReasonForeign) {
			slog.WarnContext(ctx, "confluence page belongs to another repository; not written",
				"repo", label, "page", r.Title, "reason", r.Reason)
		}

		for _, text := range r.Comments.Lost {
			slog.WarnContext(ctx, "confluence inline comment lost its anchor",
				"repo", label, "page", r.Title, "text", text)
		}
	}
}

// recordState writes a sync row for a run that wrote no pages: disabled,
// refused, running, or failed before starting. It keeps the previous run's
// folder, and a failure to write is logged rather than returned, since the
// state it records is already in the log.
func (s *Service) recordState(
	ctx context.Context, in *store.ExportInputs, sc *doczcfg.ConfluenceSyncConfig, res *Result,
) {
	p := &store.UpsertConfluenceSyncParams{
		RepoID:    in.Repo.ID,
		Status:    string(res.Status),
		Reason:    res.Reason,
		HeadSha:   res.HeadSHA,
		Counts:    json.RawMessage(`{}`),
		StartedAt: timestamp(s.now()),
	}

	if in.Sync != nil {
		p.Site, p.Space = in.Sync.Site, in.Sync.Space
		p.FolderID, p.FolderTitle, p.FolderUrl = in.Sync.FolderID, in.Sync.FolderTitle, in.Sync.FolderUrl
	}

	if sc != nil {
		p.Site, p.Space = sc.Site, sc.Space
	}

	if res.Status != StatusRunning {
		p.FinishedAt = timestamp(s.now())
	}

	if err := s.store.UpsertConfluenceSync(context.WithoutCancel(ctx), p); err != nil {
		slog.ErrorContext(ctx, "record confluence export state", "repo", res.Repo, "status", res.Status, "err", err)
	}
}

// record writes a finished run: the sync row and a row for every page with
// a page id or a previous row (DESIGN-0021 §5 step 6). It runs on an
// uncancelled context so a shutdown mid-run still records what was written.
func (s *Service) record(
	ctx context.Context, in *store.ExportInputs, sc *doczcfg.ConfluenceSyncConfig,
	res *Result, rep *confluence.Report, started time.Time,
) error {
	ctx, span := tracer.Start(context.WithoutCancel(ctx), "export.record")
	defer span.End()

	counts, err := json.Marshal(res.Counts)
	if err != nil {
		return fmt.Errorf("marshal counts: %w", err)
	}

	now := s.now()
	sync := &store.UpsertConfluenceSyncParams{
		RepoID: in.Repo.ID, Status: string(res.Status), Reason: res.Reason,
		Site: sc.Site, Space: sc.Space, HeadSha: res.HeadSHA, Counts: counts,
		StartedAt: timestamp(started), FinishedAt: timestamp(now),
	}

	if rep.Folder != nil {
		sync.FolderID, sync.FolderTitle, sync.FolderUrl = rep.Folder.ID, rep.Folder.Title, rep.Folder.WebURL
	} else if in.Sync != nil {
		sync.FolderID, sync.FolderTitle, sync.FolderUrl = in.Sync.FolderID, in.Sync.FolderTitle, in.Sync.FolderUrl
	}

	pages := make([]store.UpsertConfluencePageParams, 0, len(rep.Pages))

	for i := range rep.Pages {
		r := &rep.Pages[i]
		if r.Key == "" {
			continue
		}

		if _, known := in.PageIDs[r.Key]; r.PageID == "" && !known {
			continue
		}

		p := store.UpsertConfluencePageParams{
			RepoID: in.Repo.ID, Key: r.Key, DocID: r.ID, PageID: r.PageID, Title: r.Title, Source: r.Source, Url: r.URL,
			Version: int32Of(r.Version), Hash: r.Hash, Action: r.Action.String(), Reason: r.Reason,
			CommentsLost: int32Of(len(r.Comments.Lost)), SyncedAt: timestamp(now),
		}
		if r.Edited != nil {
			p.EditedFrom, p.EditedExpected = int32Of(r.Edited.Version), int32Of(r.Edited.Expected)
		}

		pages = append(pages, p)
	}

	if err := s.store.RecordConfluenceExport(ctx, sync, pages); err != nil {
		spanError(span, err)

		return fmt.Errorf("record export of %s: %w", res.Repo, err)
	}

	return nil
}

func timestamp(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// int32Of narrows a count or version, which never approaches the limit.
func int32Of(n int) int32 {
	if n > math.MaxInt32 {
		return math.MaxInt32
	}

	return int32(n) //nolint:gosec // bounded just above
}

func spanError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}
