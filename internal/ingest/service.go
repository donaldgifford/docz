package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"path/filepath"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/donaldgifford/docz-api/internal/search"
	"github.com/donaldgifford/docz-api/internal/store"
	doczcfg "github.com/donaldgifford/docz/pkg/doczcore/config"
	doczdoc "github.com/donaldgifford/docz/pkg/doczcore/document"
)

// tracer is the instrumentation scope for the ingest pipeline spans.
var tracer = otel.Tracer("github.com/donaldgifford/docz-api/internal/ingest")

// repoStore is the persistence surface the pipeline needs: reconcile a repo in
// one transaction, then read back the documents that changed so they can be
// pushed to the search index. *store.Store satisfies it.
type repoStore interface {
	ReconcileRepo(ctx context.Context, in *store.ReconcileInput) (store.ReconcileResult, error)
	GetDocumentsByIDs(ctx context.Context, repoID int64, docIDs []string) ([]store.Document, error)
}

// Indexer is the narrow Meilisearch surface ingest needs after a reconcile
// commit. *search.Client satisfies it. Declared here (consumer side) so ingest
// depends on the search package only for the IndexDoc boundary type.
type Indexer interface {
	IndexDocuments(ctx context.Context, docs []search.IndexDoc) error
	DeleteDocuments(ctx context.Context, ids []string) error
}

// *search.Client is the production Indexer.
var _ Indexer = (*search.Client)(nil)

// Service runs the synchronous fetch → parse → map → reconcile pipeline for one
// repo, then mirrors the reconcile's document changes into the search index.
type Service struct {
	store   repoStore
	fetcher RepoFetcher
	indexer Indexer
}

// NewService builds a Service over a store, a repo fetcher, and an optional
// indexer. A nil indexer disables search indexing (used by tests and the
// Postgres-only paths).
func NewService(st repoStore, f RepoFetcher, idx Indexer) *Service {
	return &Service{store: st, fetcher: f, indexer: idx}
}

// Run ingests one repo at HEAD: fetch, parse .docz.yaml, map its doc types and
// documents, and reconcile in one transaction (the content-hash gate lives in
// the store). A root CHANGELOG.md is cached raw on the repo row. installationID
// is recorded on the repo row for the later webhook path.
func (s *Service) Run(
	ctx context.Context, installationID int64, owner, name string,
) (store.ReconcileResult, error) {
	var zero store.ReconcileResult

	ctx, span := tracer.Start(ctx, "ingest.run", trace.WithAttributes(
		attribute.String("repo", owner+"/"+name),
		attribute.Int64("installation_id", installationID),
	))
	defer span.End()

	snap, err := s.fetchSnapshot(ctx, owner, name)
	if err != nil {
		return zero, fmt.Errorf("fetch %s/%s: %w", owner, name, err)
	}

	cfg, err := loadConfig(snap.ConfigYAML)
	if err != nil {
		return zero, fmt.Errorf("config for %s/%s: %w", owner, name, err)
	}
	warnings, err := cfg.Validate()
	if err != nil {
		return zero, fmt.Errorf("invalid .docz.yaml for %s/%s: %w", owner, name, err)
	}
	for _, w := range warnings {
		slog.Warn("docz config warning", "repo", owner+"/"+name, "warning", w)
	}

	configSnap, err := json.Marshal(cfg)
	if err != nil {
		return zero, fmt.Errorf("marshal config snapshot for %s/%s: %w", owner, name, err)
	}

	docTypes, err := buildDocTypes(&cfg)
	if err != nil {
		return zero, fmt.Errorf("doc types for %s/%s: %w", owner, name, err)
	}
	documents, err := buildDocuments(&cfg, snap.Blobs)
	if err != nil {
		return zero, fmt.Errorf("documents for %s/%s: %w", owner, name, err)
	}

	in := &store.ReconcileInput{
		Repo: store.RepoInput{
			InstallationID: installationID,
			Owner:          owner,
			Name:           name,
			DefaultBranch:  snap.DefaultBranch,
			DocsDir:        cfg.DocsDir,
			ConfigSnapshot: configSnap,
			LastSyncedSHA:  snap.HeadSHA,
			ChangelogMD:    string(snap.ChangelogMD),
			ChangelogSHA:   snap.ChangelogSHA,
		},
		DocTypes:  docTypes,
		Documents: documents,
	}

	result, err := s.reconcile(ctx, in)
	if err != nil {
		return zero, fmt.Errorf("reconcile %s/%s: %w", owner, name, err)
	}

	// Postgres is the source of truth and has committed; mirror the change set
	// into the search index best-effort (see indexSearch).
	s.indexSearch(ctx, owner, name, &result)
	return result, nil
}

// fetchSnapshot fetches the repo snapshot under a child span. It returns the
// raw fetcher error (Run wraps it with repo context).
func (s *Service) fetchSnapshot(ctx context.Context, owner, name string) (*RepoSnapshot, error) {
	ctx, span := tracer.Start(ctx, "ingest.fetch")
	defer span.End()
	snap, err := s.fetcher.Fetch(ctx, owner, name)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	return snap, nil
}

// reconcile runs the store transaction under a child span. It returns the raw
// store error (Run wraps it with repo context).
func (s *Service) reconcile(
	ctx context.Context, in *store.ReconcileInput,
) (store.ReconcileResult, error) {
	ctx, span := tracer.Start(ctx, "ingest.reconcile")
	defer span.End()
	res, err := s.store.ReconcileRepo(ctx, in)
	if err != nil {
		recordSpanError(span, err)
	}
	return res, err
}

// recordSpanError marks span as failed: it records the error as a span event
// and sets the OTel error status so failed operations aren't rendered as
// successful in trace tooling.
func recordSpanError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// indexSearch mirrors a reconcile's document changes into the search index
// after the Postgres commit. Failures are logged, not returned: Postgres has
// already committed, and the next reconcile re-indexes the affected documents
// (eventual consistency; Phase 4's queue makes this reliable). It is a no-op
// when no indexer is configured.
func (s *Service) indexSearch(ctx context.Context, owner, name string, result *store.ReconcileResult) {
	if s.indexer == nil {
		return
	}
	if err := s.syncIndex(ctx, owner, name, result); err != nil {
		slog.Error("search index sync failed; index may lag postgres",
			"repo", owner+"/"+name, "err", err)
	}
}

// syncIndex deletes removed documents from the index by primary key, then
// fetches the full rows for the upserted (new/changed) documents and indexes
// them. It reuses the reconcile's content-hash gate: only changed doc ids reach
// the index.
func (s *Service) syncIndex(
	ctx context.Context, owner, name string, result *store.ReconcileResult,
) (err error) {
	ctx, span := tracer.Start(ctx, "ingest.index")
	defer func() {
		if err != nil {
			recordSpanError(span, err)
		}
		span.End()
	}()

	if len(result.DeletedDocIDs) > 0 {
		ids := make([]string, len(result.DeletedDocIDs))
		for i, docID := range result.DeletedDocIDs {
			ids[i] = primaryKey(result.RepoID, docID)
		}
		if err := s.indexer.DeleteDocuments(ctx, ids); err != nil {
			return fmt.Errorf("delete from index: %w", err)
		}
	}

	if len(result.UpsertedDocIDs) == 0 {
		return nil
	}
	rows, err := s.store.GetDocumentsByIDs(ctx, result.RepoID, result.UpsertedDocIDs)
	if err != nil {
		return fmt.Errorf("fetch upserted docs: %w", err)
	}
	docs := make([]search.IndexDoc, 0, len(rows))
	for i := range rows {
		docs = append(docs, toIndexDoc(owner, name, result.RepoID, &rows[i]))
	}
	if err := s.indexer.IndexDocuments(ctx, docs); err != nil {
		return fmt.Errorf("index documents: %w", err)
	}
	return nil
}

// buildDocTypes maps every enabled type in the config to a store.DocTypeInput.
func buildDocTypes(cfg *doczcfg.Config) ([]store.DocTypeInput, error) {
	names := cfg.EnabledTypes()
	types := make([]store.DocTypeInput, 0, len(names))
	for _, name := range names {
		tc := cfg.Types[name]
		dt, err := mapDocType(name, &tc)
		if err != nil {
			return nil, err
		}
		types = append(types, dt)
	}
	return types, nil
}

// buildDocuments maps fetched blobs to store.DocumentInput values. A blob is
// assigned to a type by matching its directory against each enabled type's
// docs_dir/<type.dir>/. Blobs outside any type dir (over-fetched by the
// convention filter) are ignored; a blob with no frontmatter is skipped with a
// warning without aborting the repo.
func buildDocuments(cfg *doczcfg.Config, blobs []BlobEntry) ([]store.DocumentInput, error) {
	typeByDir := make(map[string]string, len(cfg.Types))
	for _, name := range cfg.EnabledTypes() {
		typeByDir[filepath.ToSlash(cfg.TypeDir(name))] = name
	}

	docs := make([]store.DocumentInput, 0, len(blobs))
	for i := range blobs {
		blob := &blobs[i]
		typeName, ok := typeByDir[path.Dir(blob.Path)]
		if !ok {
			continue
		}
		fm, err := doczdoc.ParseFrontmatter(blob.Content)
		if errors.Is(err, doczdoc.ErrNoFrontmatter) {
			slog.Warn("skipping doc without frontmatter", "path", blob.Path)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("parse frontmatter %s: %w", blob.Path, err)
		}
		doc, err := mapDocument(typeName, blob, &fm)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
}
