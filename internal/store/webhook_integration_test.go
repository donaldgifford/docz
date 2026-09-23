//go:build integration

package store

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestRecordDeliveryIdempotent proves the X-GitHub-Delivery idempotency gate:
// the first record is new, a replay of the same id is not.
func TestRecordDeliveryIdempotent(t *testing.T) {
	ctx := t.Context()
	const id = "delivery-abc-123"

	isNew, err := testStore.RecordDelivery(ctx, id, "push")
	if err != nil {
		t.Fatalf("first RecordDelivery: %v", err)
	}
	if !isNew {
		t.Error("first RecordDelivery isNew = false, want true")
	}

	isNew, err = testStore.RecordDelivery(ctx, id, "push")
	if err != nil {
		t.Fatalf("replayed RecordDelivery: %v", err)
	}
	if isNew {
		t.Error("replayed RecordDelivery isNew = true, want false (already recorded)")
	}
}

// TestDeleteRepoCascades proves DeleteRepo returns the deleted repo id, wipes
// its documents via ON DELETE CASCADE, and signals a second delete as
// pgx.ErrNoRows (already absent).
func TestDeleteRepoCascades(t *testing.T) {
	ctx := t.Context()
	seedInstallation(t, 500)

	res, err := testStore.ReconcileRepo(ctx, &ReconcileInput{
		Repo: RepoInput{
			InstallationID: 500, Owner: "acme", Name: "deleteme", DefaultBranch: "main",
			DocsDir: "docs", ConfigSnapshot: json.RawMessage(`{}`),
		},
		Documents: []DocumentInput{doc("0001", "h1")},
	})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	id, err := testStore.DeleteRepo(ctx, "acme", "deleteme")
	if err != nil {
		t.Fatalf("DeleteRepo: %v", err)
	}
	if id != res.RepoID {
		t.Errorf("DeleteRepo id = %d, want %d", id, res.RepoID)
	}

	hashes, err := testStore.q.ListDocumentHashes(ctx, res.RepoID)
	if err != nil {
		t.Fatalf("ListDocumentHashes: %v", err)
	}
	if len(hashes) != 0 {
		t.Errorf("document rows after delete = %d, want 0 (cascade)", len(hashes))
	}

	if _, err := testStore.DeleteRepo(ctx, "acme", "deleteme"); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("second DeleteRepo err = %v, want pgx.ErrNoRows", err)
	}
}

// TestDeleteInstallationCascades proves ListRepoIDsByInstallation enumerates an
// installation's repos and DeleteInstallation removes them (and their docs) via
// the repos.installation_id ON DELETE CASCADE.
func TestDeleteInstallationCascades(t *testing.T) {
	ctx := t.Context()
	seedInstallation(t, 501)

	for _, name := range []string{"repo-a", "repo-b"} {
		if _, err := testStore.ReconcileRepo(ctx, &ReconcileInput{
			Repo: RepoInput{
				InstallationID: 501, Owner: "beta", Name: name, DefaultBranch: "main",
				DocsDir: "docs", ConfigSnapshot: json.RawMessage(`{}`),
			},
			Documents: []DocumentInput{doc("0001", "h-"+name)},
		}); err != nil {
			t.Fatalf("reconcile %s: %v", name, err)
		}
	}

	ids, err := testStore.ListRepoIDsByInstallation(ctx, 501)
	if err != nil {
		t.Fatalf("ListRepoIDsByInstallation: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("repo ids = %v, want 2", ids)
	}

	if derr := testStore.DeleteInstallation(ctx, 501); derr != nil {
		t.Fatalf("DeleteInstallation: %v", derr)
	}

	if _, gerr := testStore.GetRepo(ctx, "beta", "repo-a"); !errors.Is(gerr, pgx.ErrNoRows) {
		t.Errorf("GetRepo after installation delete err = %v, want pgx.ErrNoRows", gerr)
	}
	if left, lerr := testStore.ListRepoIDsByInstallation(ctx, 501); lerr != nil || len(left) != 0 {
		t.Errorf("repos left after installation delete = %v (err %v), want none", left, lerr)
	}
}
