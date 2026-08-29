package search

// Source values distinguish the two record kinds sharing the index: docz
// documents and api-block pages (DESIGN-0004; the `source` facet).
const (
	SourceDoc  = "doc"
	SourcePage = "page"
)

// IndexDoc is one record as stored in the Meilisearch documents index. The
// ingest layer builds these from Postgres rows; the field names and JSON tags
// are the index schema. ID is the composite primary key — "<repo_id>_<doc_id>"
// for documents ("_" not ":" — Meilisearch ids allow only [a-zA-Z0-9-_]),
// "<repo_id>_p_<hash>" for pages (published paths contain characters ids
// reject, so the path is hashed). Created is a "YYYY-MM-DD" date (empty when
// unset); UpdatedAt is Unix seconds. Page records leave the doc-only fields
// (DocID/Type/Status/Author/Created) empty; Path is the published page path
// on pages and the repo-relative file path on documents, so hits of either
// kind can deep-link.
type IndexDoc struct {
	ID        string `json:"id"`
	Source    string `json:"source"`
	Repo      string `json:"repo"`
	RepoID    int64  `json:"repo_id"`
	DocID     string `json:"doc_id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Author    string `json:"author"`
	Created   string `json:"created"`
	Path      string `json:"path"`
	Body      string `json:"body"`
	UpdatedAt int64  `json:"updated_at"`
}

// SearchParams is the inbound query the httpapi layer passes to Search.
// AllowedRepoIDs is injected from the authorize seam: a non-nil slice restricts
// results to those repo ids (an empty slice yields no results); nil disables the
// repo filter entirely. Repo/Type/Status/Author are optional facet filters.
type SearchParams struct {
	Query          string
	AllowedRepoIDs []int64
	Repo           string
	Type           string
	Status         string
	Author         string
	Offset         int64
	Limit          int64
}

// SearchHit is one result row with a highlighted body snippet. Source is
// "doc" or "page"; Path is the repo-relative file path on docs and the
// published path on pages. The doc-only fields (DocID/Type/Status/Author) are
// "" on page hits — the wire's not-applicable convention.
type SearchHit struct {
	Source  string `json:"source"`
	Repo    string `json:"repo"`
	DocID   string `json:"doc_id"`
	Type    string `json:"type"`
	Title   string `json:"title"`
	Path    string `json:"path"`
	Status  string `json:"status"`
	Author  string `json:"author"`
	Snippet string `json:"snippet"`
}

// FacetMap maps one facet's values to their result counts.
type FacetMap map[string]int64

// SearchResult is the response returned to the httpapi layer, shaped to match
// the DESIGN-0001 search wire format.
type SearchResult struct {
	Query          string              `json:"query"`
	EstimatedTotal int64               `json:"estimated_total_hits"`
	Hits           []SearchHit         `json:"hits"`
	Facets         map[string]FacetMap `json:"facets"`
}
