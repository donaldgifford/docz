package webhook

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/go-github/v88/github"
	"github.com/jackc/pgx/v5"

	"github.com/donaldgifford/docz-api/internal/queue"
	"github.com/donaldgifford/docz-api/internal/store"
)

// route dispatches a parsed webhook payload to its event handler. Unhandled
// event types (ping, and anything the app is not subscribed to) are accepted
// and ignored.
func (h *Handler) route(ctx context.Context, payload any) error {
	switch ev := payload.(type) {
	case *github.InstallationEvent:
		return h.handleInstallation(ctx, ev)
	case *github.InstallationRepositoriesEvent:
		return h.handleInstallationRepos(ctx, ev)
	case *github.PushEvent:
		return h.handlePush(ctx, ev)
	case *github.ReleaseEvent:
		logRelease(ev)
		return nil
	default:
		slog.Debug("ignoring unhandled webhook event", "type", fmt.Sprintf("%T", payload))
		return nil
	}
}

// handleInstallation onboards an installation on "created" (upsert the
// installation, enqueue an ingest per granted repo) and offboards it on
// "deleted". Other actions (suspend/unsuspend/new_permissions_accepted) are
// logged and ignored.
func (h *Handler) handleInstallation(ctx context.Context, ev *github.InstallationEvent) error {
	inst := ev.GetInstallation()
	switch action := ev.GetAction(); action {
	case "created":
		if err := h.store.UpsertInstallation(ctx, installationInput(inst)); err != nil {
			return fmt.Errorf("upsert installation %d: %w", inst.GetID(), err)
		}
		return h.enqueueRepos(ctx, inst.GetID(), ev.Repositories, "onboard")
	case "deleted":
		return h.offboardInstallation(ctx, inst.GetID())
	default:
		slog.Debug("ignoring installation action", "action", action, "installation", inst.GetID())
		return nil
	}
}

// handleInstallationRepos onboards repos added to an existing installation and
// offboards repos removed from it.
func (h *Handler) handleInstallationRepos(ctx context.Context, ev *github.InstallationRepositoriesEvent) error {
	inst := ev.GetInstallation()
	switch action := ev.GetAction(); action {
	case "added":
		// The installation row should already exist, but upsert defensively so a
		// missed installation event cannot leave the enqueued ingest without its
		// required foreign key.
		if err := h.store.UpsertInstallation(ctx, installationInput(inst)); err != nil {
			return fmt.Errorf("upsert installation %d: %w", inst.GetID(), err)
		}
		return h.enqueueRepos(ctx, inst.GetID(), ev.RepositoriesAdded, "repo_added")
	case "removed":
		return h.offboardRepos(ctx, ev.RepositoriesRemoved)
	default:
		slog.Debug("ignoring installation_repositories action", "action", action)
		return nil
	}
}

// handlePush enqueues a re-ingest when a push targets the default branch and
// touches docz content. It looks the repo up to learn its docs_dir; a push for
// a repo that is not yet onboarded is skipped (the onboard ingest will catch
// HEAD).
func (h *Handler) handlePush(ctx context.Context, ev *github.PushEvent) error {
	owner := ev.GetRepo().GetOwner().GetLogin()
	name := ev.GetRepo().GetName()

	repo, err := h.store.GetRepo(ctx, owner, name)
	if errors.Is(err, pgx.ErrNoRows) {
		slog.Info("push for unknown repo; skipping until onboarded", "repo", owner+"/"+name)
		return nil
	}
	if err != nil {
		return fmt.Errorf("get repo %s/%s: %w", owner, name, err)
	}

	if !shouldIngest(ev, repo.DocsDir) {
		slog.Debug("push not relevant to docz; skipping", "repo", owner+"/"+name, "ref", ev.GetRef())
		return nil
	}

	// A full re-ingest through the queue re-fetches HEAD and reconciles the whole
	// repo: the content-hash gate keeps unchanged docs cheap and the reconcile
	// deletes docs absent from the new HEAD. Narrowing blob fetches to the push's
	// changed paths is a possible future optimization, not needed at homelab scale.
	err = h.enq.EnqueueIngest(ctx, &queue.IngestJob{
		InstallationID: ev.GetInstallation().GetID(),
		Owner:          owner,
		Name:           name,
		Reason:         "push",
	})
	if err != nil {
		return fmt.Errorf("enqueue ingest for %s/%s: %w", owner, name, err)
	}
	return nil
}

// offboardInstallation deletes an installation (CASCADE wipes its repos,
// doc_types, and documents in Postgres) and purges each repo's documents from
// the search index. Repo ids are collected before the delete, since the CASCADE
// removes the rows that name them.
func (h *Handler) offboardInstallation(ctx context.Context, installationID int64) error {
	repoIDs, err := h.store.ListRepoIDsByInstallation(ctx, installationID)
	if err != nil {
		return fmt.Errorf("list repos for installation %d: %w", installationID, err)
	}
	if err = h.store.DeleteInstallation(ctx, installationID); err != nil {
		return fmt.Errorf("delete installation %d: %w", installationID, err)
	}
	for _, repoID := range repoIDs {
		h.purgeIndex(ctx, repoID)
	}
	slog.Info("offboarded installation", "installation", installationID, "repos", len(repoIDs))
	return nil
}

// offboardRepos deletes each removed repo (CASCADE wipes its docs) and purges it
// from the search index. A repo already absent is treated as a no-op.
func (h *Handler) offboardRepos(ctx context.Context, repos []*github.Repository) error {
	for _, repo := range repos {
		owner, name, ok := ownerName(repo)
		if !ok {
			slog.Warn("skipping repo with unparseable full name", "full_name", repo.GetFullName())
			continue
		}
		repoID, err := h.store.DeleteRepo(ctx, owner, name)
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Info("repo already absent on removal", "repo", owner+"/"+name)
			continue
		}
		if err != nil {
			return fmt.Errorf("delete repo %s/%s: %w", owner, name, err)
		}
		h.purgeIndex(ctx, repoID)
	}
	return nil
}

// enqueueRepos enqueues an ingest job for each repo, tagging it with reason for
// log correlation. Repos with an unparseable full name are skipped with a
// warning rather than failing the whole batch.
func (h *Handler) enqueueRepos(
	ctx context.Context, installationID int64, repos []*github.Repository, reason string,
) error {
	for _, repo := range repos {
		owner, name, ok := ownerName(repo)
		if !ok {
			slog.Warn("skipping repo with unparseable full name", "full_name", repo.GetFullName())
			continue
		}
		if err := h.enq.EnqueueIngest(ctx, &queue.IngestJob{
			InstallationID: installationID,
			Owner:          owner,
			Name:           name,
			Reason:         reason,
		}); err != nil {
			return fmt.Errorf("enqueue ingest for %s/%s: %w", owner, name, err)
		}
	}
	return nil
}

// purgeIndex removes a repo's documents from the search index, best-effort: a
// failure is logged but does not fail the webhook. Postgres (the source of
// truth) has already dropped the rows, and a stale index entry is harmless —
// searches filter by the caller's allowed repos, and the repo is gone. It is a
// no-op when no purger is configured.
func (h *Handler) purgeIndex(ctx context.Context, repoID int64) {
	if h.purger == nil {
		return
	}
	if err := h.purger.DeleteRepoDocuments(ctx, repoID); err != nil {
		slog.Error("failed to purge repo from search index", "repo_id", repoID, "err", err)
	}
}

// logRelease records a release event and takes no other action. Release/tag
// snapshots (the versions feature) are deferred (DESIGN-0001 Open Question 12);
// keeping the subscription wired lets the feature light up without re-plumbing.
func logRelease(ev *github.ReleaseEvent) {
	slog.Info("release event received (versions feature deferred; no action)",
		"repo", ev.GetRepo().GetFullName(),
		"action", ev.GetAction(),
		"tag", ev.GetRelease().GetTagName())
}

// shouldIngest reports whether a push warrants a re-ingest: it must target the
// repo's default branch and touch either .docz.yaml or a path under docsDir. The
// changed-path set is the union of added/modified/removed across every commit in
// the push, not just the head commit (which would miss files from intermediate
// commits in a multi-commit or force push).
func shouldIngest(ev *github.PushEvent, docsDir string) bool {
	if ev.GetRef() != "refs/heads/"+ev.GetRepo().GetDefaultBranch() {
		return false
	}
	prefix := docsDir + "/"
	for _, c := range ev.Commits {
		// Added/Modified/Removed together are the commit's changed paths; scan
		// them without materializing a union, returning on the first match.
		for _, set := range [][]string{c.Added, c.Modified, c.Removed} {
			for _, p := range set {
				if p == doczConfigFile || strings.HasPrefix(p, prefix) {
					return true
				}
			}
		}
	}
	return false
}

// installationInput maps a GitHub installation to the store's boundary input.
func installationInput(inst *github.Installation) store.InstallationInput {
	acct := inst.GetAccount()
	return store.InstallationInput{
		ID:           inst.GetID(),
		AccountLogin: acct.GetLogin(),
		AccountType:  acct.GetType(),
	}
}

// ownerName splits a repo's "owner/name" full name. Installation event payloads
// carry full_name but not always a nested owner object, so full_name is the
// reliable source. ok is false when the full name is not "owner/name" shaped.
func ownerName(repo *github.Repository) (owner, name string, ok bool) {
	owner, name, found := strings.Cut(repo.GetFullName(), "/")
	return owner, name, found && owner != "" && name != ""
}
