package search

// IndexDoc is one document as stored in the Meilisearch documents index. The
// ingest layer builds these from Postgres rows; the field names and JSON tags
// are the index schema. ID is the composite primary key "<repo_id>:<doc_id>".
// Created is a "YYYY-MM-DD" date (empty when unset); UpdatedAt is Unix seconds.
type IndexDoc struct {
	ID        string `json:"id"`
	Repo      string `json:"repo"`
	RepoID    int64  `json:"repo_id"`
	DocID     string `json:"doc_id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Author    string `json:"author"`
	Created   string `json:"created"`
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

// SearchHit is one result row with a highlighted body snippet.
type SearchHit struct {
	Repo    string `json:"repo"`
	DocID   string `json:"doc_id"`
	Type    string `json:"type"`
	Title   string `json:"title"`
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
