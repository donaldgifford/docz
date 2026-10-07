package main

import (
	"context"
	"fmt"

	"github.com/donaldgifford/docz/v2/internal/config"
	"github.com/donaldgifford/docz/v2/internal/githubapp"
	"github.com/donaldgifford/docz/v2/internal/ingest"
	"github.com/donaldgifford/docz/v2/internal/queue"
	"github.com/donaldgifford/docz/v2/internal/search"
	"github.com/donaldgifford/docz/v2/internal/store"
)

// ingestRunner adapts the ingest pipeline to queue.Ingestor. It builds a
// per-installation GitHub client for each job, so one worker serves every
// installation without being pinned to one at construction time. Building a
// client per job is cheap (ghinstallation caches the JWT) and jobs are
// infrequent (debounced, content-hash gated).
type ingestRunner struct {
	store   *store.Store
	indexer *search.Client
	github  config.GitHubConfig
	// exporter enqueues a Confluence export after each ingest; nil when the
	// server has no Atlassian credential.
	exporter ingest.Exporter
}

// *ingestRunner is the production queue.Ingestor.
var _ queue.Ingestor = (*ingestRunner)(nil)

// Run builds a GitHub client for installationID and runs one ingest of
// owner/name through the full fetch → parse → reconcile → index pipeline.
func (r *ingestRunner) Run(
	ctx context.Context, installationID int64, owner, name string,
) (store.ReconcileResult, error) {
	ghClient, err := githubapp.NewClient(
		r.github.AppID,
		[]byte(r.github.PrivateKey.Reveal()),
		r.github.APIBase,
		installationID,
	)
	if err != nil {
		return store.ReconcileResult{}, fmt.Errorf("build github client for installation %d: %w", installationID, err)
	}
	svc := ingest.NewService(r.store, ghClient, r.indexer)
	if r.exporter != nil {
		svc.WithExporter(r.exporter)
	}
	return svc.Run(ctx, installationID, owner, name)
}
