package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/donaldgifford/docz-api/internal/authorize"
	"github.com/donaldgifford/docz-api/internal/store"
)

// errStore fails every read, exercising the handlers' 500 path (serverError).
type errStore struct{}

var errBoom = errors.New("boom")

func (errStore) ListRepos(context.Context) ([]store.Repo, error) { return nil, errBoom }

func (errStore) GetRepo(context.Context, string, string) (store.Repo, error) {
	return store.Repo{}, errBoom
}

func (errStore) GetDocTypesForRepo(context.Context, int64) ([]store.DocType, error) {
	return nil, errBoom
}

func (errStore) ListDocumentsByType(
	context.Context, int64, string,
) ([]store.ListDocumentsByTypeRow, error) {
	return nil, errBoom
}

func (errStore) GetDocumentByID(context.Context, int64, string) (store.Document, error) {
	return store.Document{}, errBoom
}

func TestStoreErrorIs500(t *testing.T) {
	st := errStore{}
	// The authorizer must not itself fail, so allow everything.
	srv := testServer(st, fixedAuthorizer{allowed: authorize.AllowedRepos{1}})

	rec := doGet(t, srv, "/api/v1/repos")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 when the store errors", rec.Code)
	}
	// The 500 body is the opaque error envelope, never the underlying error.
	body := rec.Body.String()
	if !strings.Contains(body, `"error"`) || strings.Contains(body, "boom") {
		t.Errorf("body = %q, want an opaque error envelope with no internal detail", body)
	}
}
