package httpapi

import (
	"net/http"
	"strconv"

	"github.com/donaldgifford/docz-api/internal/authorize"
	"github.com/donaldgifford/docz-api/internal/search"
)

// searchDocs handles GET /api/v1/search: full-text query q with optional
// repo/type/status/author facet filters. The route is always mounted behind the
// authorize middleware, so the caller's allowed-repo set is present in context;
// it is injected as a repo_id filter so results never cross the authorization
// boundary.
func (h *Handler) searchDocs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	params := search.SearchParams{
		Query:          q.Get("q"),
		AllowedRepoIDs: authorize.FromContext(r.Context()),
		Repo:           q.Get("repo"),
		Type:           q.Get("type"),
		Status:         q.Get("status"),
		Author:         q.Get("author"),
		Offset:         parseNonNegInt(q.Get("offset")),
		Limit:          parseNonNegInt(q.Get("limit")),
	}

	result, err := h.searcher.Search(r.Context(), &params)
	if err != nil {
		serverError(w, "search", err)
		return
	}
	writeJSON(w, result)
}

// parseNonNegInt parses s as a non-negative int64, returning 0 for empty or
// invalid input (the search layer applies its own default limit).
func parseNonNegInt(s string) int64 {
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
