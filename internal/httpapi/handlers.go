package httpapi

import (
	"errors"
	"net/http"

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
