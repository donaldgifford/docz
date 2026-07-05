// Package httpapi serves the docz-api read endpoints under /api/v1: onboarded
// repos, repo detail, doc types, and documents. Handlers read through a narrow
// store interface, resolve the {type} URL segment by name/id_prefix/alias, and
// map rows to the DESIGN-0001 wire DTOs. Every route runs behind the authorize
// middleware and hides repos outside the caller's allowed set as 404s.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/donaldgifford/docz-api/internal/authorize"
	"github.com/donaldgifford/docz-api/internal/search"
	"github.com/donaldgifford/docz-api/internal/store"
)

// storeReader is the read surface httpapi needs. *store.Store satisfies it.
type storeReader interface {
	ListRepos(ctx context.Context) ([]store.Repo, error)
	GetRepo(ctx context.Context, owner, name string) (store.Repo, error)
	GetDocTypesForRepo(ctx context.Context, repoID int64) ([]store.DocType, error)
	ListDocumentsByType(ctx context.Context, repoID int64, typeName string) ([]store.ListDocumentsByTypeRow, error)
	GetDocumentByID(ctx context.Context, repoID int64, docID string) (store.Document, error)
}

// Searcher is the search surface httpapi needs. *search.Client satisfies it.
type Searcher interface {
	Search(ctx context.Context, p *search.SearchParams) (search.SearchResult, error)
}

// *search.Client is the production Searcher.
var _ Searcher = (*search.Client)(nil)

// Handler serves the /api/v1 read and search routes. searcher is optional: when
// nil, the /search route is not mounted.
type Handler struct {
	store    storeReader
	searcher Searcher
}

// NewHandler builds a Handler over a store reader, without search.
func NewHandler(st storeReader) *Handler {
	return &Handler{store: st}
}

// NewHandlerWithSearch builds a Handler with both the store reader and a
// searcher, enabling the /search route.
func NewHandlerWithSearch(st storeReader, s Searcher) *Handler {
	return &Handler{store: st, searcher: s}
}

// Mount registers the read routes on r behind the authorize middleware. Liveness
// and readiness probes are mounted elsewhere and bypass authorization. The
// /search route is registered only when a searcher was provided.
func (h *Handler) Mount(r chi.Router, authz func(http.Handler) http.Handler) {
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(authz)
		r.Get("/repos", h.listRepos)
		r.Route("/repos/{owner}/{name}", func(r chi.Router) {
			r.Get("/", h.getRepo)
			r.Get("/types", h.listTypes)
			r.Get("/types/{type}/docs", h.listDocs)
			r.Get("/types/{type}/docs/{doc_id}", h.getDoc)
		})
		if h.searcher != nil {
			r.Get("/search", h.searchDocs)
		}
	})
}

// resolveRepo loads the URL's repo and enforces the allowed-repo set, writing a
// 404 (for both missing and unauthorized repos, to hide existence) and
// returning ok=false when the caller should stop.
func (h *Handler) resolveRepo(w http.ResponseWriter, r *http.Request) (store.Repo, bool) {
	owner := chi.URLParam(r, "owner")
	name := chi.URLParam(r, "name")

	repo, err := h.store.GetRepo(r.Context(), owner, name)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "repo not found")
		return store.Repo{}, false
	}
	if err != nil {
		serverError(w, "get repo", err)
		return store.Repo{}, false
	}
	if !authorize.FromContext(r.Context()).Contains(repo.ID) {
		writeError(w, http.StatusNotFound, "repo not found")
		return store.Repo{}, false
	}
	return repo, true
}

// writeJSON serializes v as a 200 JSON response. Non-200 outcomes go through
// writeError.
func writeJSON(w http.ResponseWriter, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encoding response")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, werr := w.Write(body); werr != nil {
		slog.Debug("response write failed", "err", werr)
	}
}

// writeError writes a JSON error envelope with the given status.
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		slog.Debug("error response write failed", "status", status, "err", err)
	}
}

// serverError logs the underlying error and returns an opaque 500.
func serverError(w http.ResponseWriter, op string, err error) {
	slog.Error("httpapi server error", "op", op, "err", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}
