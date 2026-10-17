//go:build integration

package store

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestReconcileRepoAccessScopes round-trips repos.access_scopes (DESIGN-0023
// §7): the manifest's list is stored, an empty list is the default, and
// ListRepoIDsForScopes answers through the JSONB ?| operator.
func TestReconcileRepoAccessScopes(t *testing.T) {
	ctx := t.Context()
	seedInstallation(t, 501)

	reconcile := func(name string, scopes []string) int64 {
		t.Helper()
		res, err := testStore.ReconcileRepo(ctx, &ReconcileInput{Repo: RepoInput{
			InstallationID: 501, Owner: "acme", Name: name, DefaultBranch: "main",
			DocsDir: "docs", ConfigSnapshot: json.RawMessage(`{}`), AccessScopes: scopes,
		}})
		if err != nil {
			t.Fatalf("reconcile %s: %v", name, err)
		}
		return res.RepoID
	}

	publicID := reconcile("scopes-public", nil)
	teamID := reconcile("scopes-team", []string{"platform-team:read", "confluence:write"})

	repo, err := testStore.q.GetRepoByOwnerName(ctx, GetRepoByOwnerNameParams{Owner: "acme", Name: "scopes-public"})
	if err != nil {
		t.Fatalf("GetRepoByOwnerName: %v", err)
	}
	if string(repo.AccessScopes) != defaultAccessScopes {
		t.Errorf("empty AccessScopes stored as %s, want %s", repo.AccessScopes, defaultAccessScopes)
	}

	tests := []struct {
		name   string
		scopes []string
		want   map[int64]bool
	}{
		{name: "public:read unlocks the default repo only", scopes: []string{"public:read"}, want: map[int64]bool{publicID: true}},
		{name: "the team scope unlocks the team repo only", scopes: []string{"platform-team:read"}, want: map[int64]bool{teamID: true}},
		{name: "any one scope is enough", scopes: []string{"public:read", "confluence:write"}, want: map[int64]bool{publicID: true, teamID: true}},
		{name: "no scopes unlock nothing", scopes: nil, want: map[int64]bool{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids, err := testStore.q.ListRepoIDsForScopes(ctx, tt.scopes)
			if err != nil {
				t.Fatalf("ListRepoIDsForScopes: %v", err)
			}
			got := map[int64]bool{}
			for _, id := range ids {
				if id == publicID || id == teamID {
					got[id] = true
				}
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ListRepoIDsForScopes(%q) = %v, want %v", tt.scopes, got, tt.want)
			}
			for id := range tt.want {
				if !got[id] {
					t.Errorf("ListRepoIDsForScopes(%q) is missing %d", tt.scopes, id)
				}
			}
		})
	}

	// Re-ingesting with a narrower list replaces the stored one.
	reconcile("scopes-team", []string{"platform-team:read"})
	ids, err := testStore.q.ListRepoIDsForScopes(ctx, []string{"confluence:write"})
	if err != nil {
		t.Fatalf("ListRepoIDsForScopes: %v", err)
	}
	for _, id := range ids {
		if id == teamID {
			t.Error("confluence:write still unlocks the team repo after the manifest dropped it")
		}
	}
}

// TestRefreshTokensAndConsents exercises the oauth tables through the
// generated queries: rotation is one statement that only a live token can
// perform, a used token cannot rotate again, revocation covers a family,
// and a consent upserts in place.
func TestRefreshTokensAndConsents(t *testing.T) {
	ctx := t.Context()

	userID, err := testStore.UpsertUser(ctx, UserInput{Provider: "github", Subject: "oauth-1", Email: "a@example.com", Login: "a"})
	if err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}

	const client = "https://client.example/oauth.json"
	first, err := testStore.q.InsertRefreshToken(ctx, InsertRefreshTokenParams{
		TokenHash: "hash-1", FamilyID: "fam-1", UserID: userID, ClientID: client, Scope: "public:read",
	})
	if err != nil {
		t.Fatalf("InsertRefreshToken: %v", err)
	}
	if first.LastUsedAt.Valid || first.RevokedAt.Valid {
		t.Errorf("a fresh token is used=%v revoked=%v, want neither", first.LastUsedAt.Valid, first.RevokedAt.Valid)
	}

	second, err := testStore.q.RotateRefreshToken(ctx, RotateRefreshTokenParams{OldHash: "hash-1", NewHash: "hash-2", Scope: "public:read team:read"})
	if err != nil {
		t.Fatalf("RotateRefreshToken: %v", err)
	}
	if second.FamilyID != "fam-1" || second.UserID != userID || second.ClientID != client || second.Scope != "public:read team:read" {
		t.Errorf("rotated token = %+v, want the family, user, and client carried and the new scope", second)
	}

	old, err := testStore.q.GetRefreshTokenByHash(ctx, "hash-1")
	if err != nil {
		t.Fatalf("GetRefreshTokenByHash: %v", err)
	}
	if !old.LastUsedAt.Valid {
		t.Error("the rotated-out token is not marked used")
	}

	if _, err := testStore.q.RotateRefreshToken(ctx, RotateRefreshTokenParams{OldHash: "hash-1", NewHash: "hash-3", Scope: ""}); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("rotating a used token = %v, want pgx.ErrNoRows", err)
	}

	n, err := testStore.q.RevokeRefreshFamily(ctx, "fam-1")
	if err != nil {
		t.Fatalf("RevokeRefreshFamily: %v", err)
	}
	if n != 2 {
		t.Errorf("RevokeRefreshFamily revoked %d rows, want 2", n)
	}
	if _, err := testStore.q.RotateRefreshToken(ctx, RotateRefreshTokenParams{OldHash: "hash-2", NewHash: "hash-4", Scope: ""}); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("rotating a revoked token = %v, want pgx.ErrNoRows", err)
	}

	if err := testStore.q.UpsertConsent(ctx, UpsertConsentParams{UserID: userID, ClientID: client, Scope: "public:read"}); err != nil {
		t.Fatalf("UpsertConsent: %v", err)
	}
	if err := testStore.q.UpsertConsent(ctx, UpsertConsentParams{UserID: userID, ClientID: client, Scope: "public:read team:read"}); err != nil {
		t.Fatalf("UpsertConsent again: %v", err)
	}
	consent, err := testStore.q.GetConsent(ctx, GetConsentParams{UserID: userID, ClientID: client})
	if err != nil {
		t.Fatalf("GetConsent: %v", err)
	}
	if consent.Scope != "public:read team:read" {
		t.Errorf("consent scope = %q, want the upserted one", consent.Scope)
	}
	if _, err := testStore.q.GetConsent(ctx, GetConsentParams{UserID: userID, ClientID: "https://other.example/c.json"}); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("GetConsent for an unknown client = %v, want pgx.ErrNoRows", err)
	}
}
