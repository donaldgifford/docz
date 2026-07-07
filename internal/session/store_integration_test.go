//go:build integration

// Package session integration tests run the store against a real Redis
// (testcontainers): a session issued is looked up with its identity intact,
// revoked sessions and short TTLs both surface as ErrSessionNotFound.
package session

import (
	"context"
	"errors"
	"log"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/donaldgifford/docz-api/internal/auth"
)

var redisURL string

func TestMain(m *testing.M) {
	os.Exit(runMain(m))
}

func runMain(m *testing.M) int {
	ctx := context.Background()

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:7-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForLog("Ready to accept connections").WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		log.Printf("start redis: %v", err)
		return 1
	}
	defer func() {
		if terr := ctr.Terminate(ctx); terr != nil {
			log.Printf("terminate redis: %v", terr)
		}
	}()

	host, err := ctr.Host(ctx)
	if err != nil {
		log.Printf("redis host: %v", err)
		return 1
	}
	port, err := ctr.MappedPort(ctx, "6379/tcp")
	if err != nil {
		log.Printf("redis port: %v", err)
		return 1
	}
	redisURL = "redis://" + host + ":" + port.Port()

	return m.Run()
}

func newStore(t *testing.T, ttl time.Duration) *Store {
	t.Helper()
	store, err := New(redisURL, ttl, false)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() {
		if cerr := store.Close(); cerr != nil {
			t.Logf("close store: %v", cerr)
		}
	})
	return store
}

func TestIssueLookupRevoke(t *testing.T) {
	store := newStore(t, time.Hour)
	ctx := t.Context()
	identity := auth.Identity{
		Provider: "okta", Subject: "user-123", Email: "a@b.co",
		Login: "", Groups: []string{"eng", "admin"},
	}

	id, err := store.Issue(ctx, &identity)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	got, err := store.Lookup(ctx, id)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if got.Identity.Subject != "user-123" || got.Identity.Provider != "okta" {
		t.Errorf("identity = %+v, want okta/user-123", got.Identity)
	}
	if len(got.Identity.Groups) != 2 || got.Identity.Groups[0] != "eng" {
		t.Errorf("groups = %v, want [eng admin] (persisted for future authZ)", got.Identity.Groups)
	}

	if rerr := store.Revoke(ctx, id); rerr != nil {
		t.Fatalf("Revoke: %v", rerr)
	}
	if _, lerr := store.Lookup(ctx, id); !errors.Is(lerr, ErrSessionNotFound) {
		t.Errorf("Lookup after revoke = %v, want ErrSessionNotFound", lerr)
	}
}

func TestLookupUnknownIsNotFound(t *testing.T) {
	store := newStore(t, time.Hour)
	if _, err := store.Lookup(t.Context(), "never-issued"); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("Lookup unknown = %v, want ErrSessionNotFound", err)
	}
}

func TestSessionExpires(t *testing.T) {
	store := newStore(t, 300*time.Millisecond)
	ctx := t.Context()

	id, err := store.Issue(ctx, &auth.Identity{Provider: "github", Subject: "1"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	time.Sleep(600 * time.Millisecond)
	if _, lerr := store.Lookup(ctx, id); !errors.Is(lerr, ErrSessionNotFound) {
		t.Errorf("Lookup after TTL = %v, want ErrSessionNotFound", lerr)
	}
}
