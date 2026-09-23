package search

import (
	"errors"
	"fmt"
)

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

// Sort tokens accepted by Search, spelled "<attribute>:<direction>" over the
// index's sortable attributes. This set is the contract's enum for the sort
// query parameter, so a generated client gets a union type and cannot
// misspell a value. Direction is always explicit: a bare attribute would need
// a documented default, and the enum exists to make every accepted value
// visible.
const (
	SortUpdatedDesc = "updated_at:desc"
	SortUpdatedAsc  = "updated_at:asc"
	SortCreatedDesc = "created:desc"
	SortCreatedAsc  = "created:asc"
)

// sortSecondary maps each accepted sort token to the tie-breaking key sent
// after it — the other sortable attribute, in the same direction.
//
// The secondary key is not cosmetic. One reconcile is one Postgres
// transaction and `now()` is the transaction timestamp, so every record a
// repository's ingest touches shares an updated_at to the second; a first
// onboard makes a whole repository tie. Without a secondary key those ties
// would fall through to relevance and then Meilisearch's internal order,
// which is stable but arbitrary, and a freshly onboarded registry sorted
// "newest first" would look unsorted (DESIGN-0005).
//
// The map's keys double as the accepted-token allowlist (see ParseSort), so
// the two cannot drift apart.
var sortSecondary = map[string]string{
	SortUpdatedDesc: SortCreatedDesc,
	SortUpdatedAsc:  SortCreatedAsc,
	SortCreatedDesc: SortUpdatedDesc,
	SortCreatedAsc:  SortUpdatedAsc,
}

// ErrInvalidSort reports a sort token outside the accepted set.
var ErrInvalidSort = errors.New("invalid sort")

// ParseSort validates a sort token from a caller. An empty string is the
// unsorted default and parses to itself; an accepted token parses to itself;
// anything else is ErrInvalidSort.
//
// Unknown tokens are rejected rather than ignored. A silently dropped sort
// returns results in an order the caller did not ask for and cannot detect,
// which is the confusion the ranking-rule placement exists to prevent. The
// lenient parsing the offset and limit parameters get is about supplying a
// default for a missing value, not about swallowing a malformed one — and a
// sort has no meaningful default to fall back to.
func ParseSort(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	if _, ok := sortSecondary[s]; !ok {
		return "", fmt.Errorf("%w: %q", ErrInvalidSort, s)
	}
	return s, nil
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
	// Source narrows to one record kind, SourceDoc or SourcePage; "" returns
	// both. Like the other facet filters it is not validated — an unknown
	// value simply matches nothing.
	Source string
	// Sort is a ParseSort-validated token, or "" to rank by relevance.
	// Search does not re-validate it: an unrecognized value would reach
	// Meilisearch as an invalid sort expression and fail the whole query.
	Sort   string
	Offset int64
	Limit  int64
}

// SearchHit is one result row with a highlighted body snippet. Source is
// "doc" or "page"; Path is the repo-relative file path on docs and the
// published path on pages. The doc-only fields (DocID/Type/Status/Author/
// Created) are "" on page hits — the wire's not-applicable convention.
//
// The two dates are distinct and both mirror their Document counterparts:
// Created is the author-typed frontmatter date ("YYYY-MM-DD"), while
// UpdatedAt is when docz-api last ingested a content change for the record
// (RFC3339, UTC) — not the git commit time, and not empty on page hits,
// which carry a real repo_pages stamp. A first ingest stamps every record
// in the repo at onboard time, because one reconcile is one transaction
// (DESIGN-0005).
type SearchHit struct {
	Source    string `json:"source"`
	Repo      string `json:"repo"`
	DocID     string `json:"doc_id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Path      string `json:"path"`
	Status    string `json:"status"`
	Author    string `json:"author"`
	Created   string `json:"created"`
	UpdatedAt string `json:"updated_at"`
	Snippet   string `json:"snippet"`
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
