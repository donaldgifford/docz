package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/donaldgifford/docz-api/internal/authorize"
	"github.com/donaldgifford/docz-api/internal/store"
)

// listRepos returns the onboarded repos visible to the caller.
func (h *Handler) listRepos(w http.ResponseWriter, r *http.Request) {
	repos, err := h.store.ListRepos(r.Context())
	if err != nil {
		serverError(w, "list repos", err)
		return
	}
	allowed := authorize.FromContext(r.Context())

	out := make([]repoSummaryDTO, 0, len(repos))
	for i := range repos {
		if allowed.Contains(repos[i].ID) {
			out = append(out, toRepoSummary(&repos[i]))
		}
	}
	writeJSON(w, map[string]any{"repos": out})
}

// getRepo returns one repo's detail, including its config snapshot and types.
func (h *Handler) getRepo(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	types, err := h.store.GetDocTypesForRepo(r.Context(), repo.ID)
	if err != nil {
		serverError(w, "get doc types", err)
		return
	}
	writeJSON(w, toRepoDetail(&repo, types))
}

// getRepoIndex returns the cached docs_dir/index.md repo home (DESIGN-0003).
// Presence keys off index_sha: an empty-but-present file stores a NULL body
// with a valid sha, so it serves 200 with an empty index_md, while a repo
// with no index.md at HEAD is a 404.
func (h *Handler) getRepoIndex(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	if !repo.IndexSha.Valid {
		writeError(w, http.StatusNotFound, "index not found")
		return
	}
	writeJSON(w, toRepoIndex(&repo))
}

// getRepoChangelog returns a repo's cached changelog. A repo that has not
// enabled the .docz.yaml changelog: block has no cached sha and 404s.
func (h *Handler) getRepoChangelog(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	if !repo.ChangelogSha.Valid {
		writeError(w, http.StatusNotFound, "changelog not found")
		return
	}
	writeJSON(w, toRepoChangelog(&repo))
}

// listRepoPages returns a repo's published pages (metadata only), ordered by
// path. A repo with no pages — including every repo without an enabled api:
// block — returns an empty list, not 404: the repo exists; its page set is
// empty (DESIGN-0004; existence hiding stays at the repo level).
func (h *Handler) listRepoPages(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	pages, err := h.store.ListRepoPages(r.Context(), repo.ID)
	if err != nil {
		serverError(w, "list pages", err)
		return
	}
	out := make([]pageSummaryDTO, len(pages))
	for i := range pages {
		out[i] = toPageSummary(&pages[i])
	}
	writeJSON(w, map[string]any{"pages": out})
}

// getRepoPage returns one page (including raw markdown) by its published
// path, taken from the chi wildcard. The decoded value is re-validated before
// lookup — docz's config validation explicitly does not survive URL decoding,
// so this check is load-bearing (DESIGN-0004) — and rejected as 404, not 400:
// an invalid path is definitionally not a page, and existence hiding argues
// for one indistinguishable miss. The lookup itself is exact-byte.
func (h *Handler) getRepoPage(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	p, err := url.PathUnescape(chi.URLParam(r, "*"))
	if err != nil || !validPagePath(p) {
		writeError(w, http.StatusNotFound, "page not found")
		return
	}
	page, err := h.store.GetRepoPageByPath(r.Context(), repo.ID, p)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "page not found")
		return
	}
	if err != nil {
		serverError(w, "get page", err)
		return
	}
	writeJSON(w, toPage(repoLabel(&repo), &page))
}

// validPagePath reports whether a decoded pages wildcard is a lookup-safe
// published path: non-empty, relative, forward-slash separated, no "." / ".."
// / empty segments, no backslashes, no control bytes.
func validPagePath(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") || strings.ContainsRune(p, '\\') {
		return false
	}
	for _, r := range p {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// listTypes returns a repo's doc types.
func (h *Handler) listTypes(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}
	types, err := h.store.GetDocTypesForRepo(r.Context(), repo.ID)
	if err != nil {
		serverError(w, "get doc types", err)
		return
	}
	writeJSON(w, map[string]any{"types": toTypeDTOs(types)})
}

// listDocs returns a repo's documents of one type (metadata only). The {type}
// segment is resolved by name/id_prefix/alias.
func (h *Handler) listDocs(w http.ResponseWriter, r *http.Request) {
	repo, canonical, ok := h.resolveRepoType(w, r)
	if !ok {
		return
	}
	docs, err := h.store.ListDocumentsByType(r.Context(), repo.ID, canonical)
	if err != nil {
		serverError(w, "list documents", err)
		return
	}
	label := repoLabel(&repo)
	out := make([]documentDTO, len(docs))
	for i := range docs {
		out[i] = toDocumentSummary(label, &docs[i])
	}
	writeJSON(w, map[string]any{"docs": out})
}

// getDoc returns one document, including its raw markdown.
func (h *Handler) getDoc(w http.ResponseWriter, r *http.Request) {
	repo, canonical, ok := h.resolveRepoType(w, r)
	if !ok {
		return
	}
	docID := chi.URLParam(r, "doc_id")
	doc, err := h.store.GetDocumentByID(r.Context(), repo.ID, docID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if err != nil {
		serverError(w, "get document", err)
		return
	}
	// The doc id is unique per repo; if it belongs to a different type than the
	// URL names, treat it as not found under this path.
	if doc.Type != canonical {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	writeJSON(w, toDocument(repoLabel(&repo), &doc))
}

// resolveRepoType resolves the repo (with authorization) and the {type} segment
// to its canonical name, writing a 404 and returning ok=false on any miss.
func (h *Handler) resolveRepoType(w http.ResponseWriter, r *http.Request) (repo store.Repo, canonical string, ok bool) {
	repo, ok = h.resolveRepo(w, r)
	if !ok {
		return store.Repo{}, "", false
	}
	types, err := h.store.GetDocTypesForRepo(r.Context(), repo.ID)
	if err != nil {
		serverError(w, "get doc types", err)
		return store.Repo{}, "", false
	}
	canonical, ok = resolveType(types, chi.URLParam(r, "type"))
	if !ok {
		writeError(w, http.StatusNotFound, "type not found")
		return store.Repo{}, "", false
	}
	return repo, canonical, true
}
