package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"path/filepath"

	"github.com/donaldgifford/docz-api/internal/store"
	doczcfg "github.com/donaldgifford/docz/pkg/doczcore/config"
	doczdoc "github.com/donaldgifford/docz/pkg/doczcore/document"
)

// reconciler is the narrow store surface the pipeline needs. *store.Store
// satisfies it.
type reconciler interface {
	ReconcileRepo(ctx context.Context, in *store.ReconcileInput) (store.ReconcileResult, error)
}

// Service runs the synchronous fetch → parse → map → reconcile pipeline for one
// repo.
type Service struct {
	store   reconciler
	fetcher RepoFetcher
}

// NewService builds a Service over a store and a repo fetcher.
func NewService(st reconciler, f RepoFetcher) *Service {
	return &Service{store: st, fetcher: f}
}

// Run ingests one repo at HEAD: fetch, parse .docz.yaml, map its doc types and
// documents, and reconcile in one transaction (the content-hash gate lives in
// the store). A root CHANGELOG.md is cached raw on the repo row. installationID
// is recorded on the repo row for the later webhook path.
func (s *Service) Run(
	ctx context.Context, installationID int64, owner, name string,
) (store.ReconcileResult, error) {
	var zero store.ReconcileResult

	snap, err := s.fetcher.Fetch(ctx, owner, name)
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
		return zero, err
	}
	documents, err := buildDocuments(&cfg, snap.Blobs)
	if err != nil {
		return zero, err
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

	result, err := s.store.ReconcileRepo(ctx, in)
	if err != nil {
		return zero, fmt.Errorf("reconcile %s/%s: %w", owner, name, err)
	}
	return result, nil
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
