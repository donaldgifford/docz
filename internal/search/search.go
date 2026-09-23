package search

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/meilisearch/meilisearch-go"
)

// Search tuning knobs.
const (
	// defaultSearchLimit caps hits when the caller does not specify a limit.
	defaultSearchLimit = 20
	// snippetCropLength is the word window Meilisearch crops the body snippet to.
	snippetCropLength = 40
)

// Body-highlight tags wrapped around matched terms in the snippet.
const (
	highlightPreTag  = "<em>"
	highlightPostTag = "</em>"
)

// facetNames are the attributes faceted (and returned as counts) on every search.
var facetNames = []string{"repo", "type", "status", "author", "source"}

// retrieveAttributes are the index attributes Meilisearch returns on a hit.
// It is the first place a hit field can be dropped: an attribute missing here
// decodes as a zero value however rawHit and decodeHits are written, and no
// test that fakes the searcher can see the difference (INV-0009 F2). A new
// SearchHit field needs all three — this list, rawHit, and decodeHits.
var retrieveAttributes = []string{
	"source", "repo", "doc_id", "type", "title", "path",
	"status", "author", "created", "updated_at", "body",
}

// Search runs a full-text query with facet filters and returns hits, facet
// counts, and highlighted snippets. The authorize seam's AllowedRepoIDs is
// injected as a repo_id filter; Repo/Type/Status/Author narrow the results
// further. An empty Query matches everything (subject to the filters).
func (c *Client) Search(ctx context.Context, p *SearchParams) (SearchResult, error) {
	resp, err := c.svc.Index(indexUID).SearchWithContext(ctx, p.Query, buildSearchRequest(p))
	if err != nil {
		return SearchResult{}, fmt.Errorf("meilisearch search: %w", err)
	}

	hits, err := decodeHits(resp.Hits)
	if err != nil {
		return SearchResult{}, err
	}
	facets, err := parseFacets(resp.FacetDistribution)
	if err != nil {
		return SearchResult{}, err
	}

	return SearchResult{
		Query:          p.Query,
		EstimatedTotal: resp.EstimatedTotalHits,
		Hits:           hits,
		Facets:         facets,
	}, nil
}

// buildSearchRequest assembles the Meilisearch request for one search. It is
// separated from Search so the request can be asserted without a server: the
// Filter and Sort keys are both conditional, and the promise that an unsorted
// or unfiltered search sends the same request it always has is only checkable
// on the assembled value.
func buildSearchRequest(p *SearchParams) *meilisearch.SearchRequest {
	limit := p.Limit
	if limit <= 0 {
		limit = defaultSearchLimit
	}

	req := &meilisearch.SearchRequest{
		Offset:                p.Offset,
		Limit:                 limit,
		Facets:                facetNames,
		AttributesToRetrieve:  retrieveAttributes,
		AttributesToCrop:      []string{"body"},
		CropLength:            snippetCropLength,
		AttributesToHighlight: []string{"body"},
		HighlightPreTag:       highlightPreTag,
		HighlightPostTag:      highlightPostTag,
	}
	// Set Filter only when non-empty: an empty filter string is not valid.
	if f := buildFilter(p); f != "" {
		req.Filter = f
	}
	// Leave Sort nil when unsorted, so an unsorted search is the same request
	// Meilisearch received before sorting existed and the sort ranking rule
	// stays inert.
	if p.Sort != "" {
		req.Sort = sortKeys(p.Sort)
	}
	return req
}

// sortKeys returns the Meilisearch sort expression for a validated token:
// the requested key followed by its tie-breaking secondary. An unrecognized
// token yields nil, which leaves the search unsorted rather than failing it —
// unreachable through the handler, which rejects such a token with a 400
// before Search is called.
func sortKeys(token string) []string {
	secondary, ok := sortSecondary[token]
	if !ok {
		return nil
	}
	return []string{token, secondary}
}

// rawHit is the decode target for one Meilisearch hit. _formatted carries the
// cropped, highlighted body used as the result snippet.
type rawHit struct {
	Source    string       `json:"source"`
	Repo      string       `json:"repo"`
	DocID     string       `json:"doc_id"`
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Path      string       `json:"path"`
	Status    string       `json:"status"`
	Author    string       `json:"author"`
	Created   string       `json:"created"`
	UpdatedAt int64        `json:"updated_at"` // Unix seconds, the index schema.
	Formatted rawFormatted `json:"_formatted"`
}

// rawFormatted holds the highlighted/cropped fields of a hit.
type rawFormatted struct {
	Body string `json:"body"`
}

// decodeHits converts Meilisearch hits into the wire SearchHit slice, taking the
// snippet from the cropped, highlighted body. The result is non-nil so it
// serializes as [] when empty.
func decodeHits(h meilisearch.Hits) ([]SearchHit, error) {
	var raws []rawHit
	if err := h.DecodeInto(&raws); err != nil {
		return nil, fmt.Errorf("decode search hits: %w", err)
	}
	hits := make([]SearchHit, len(raws))
	for i := range raws {
		r := &raws[i]
		hits[i] = SearchHit{
			Source:    r.Source,
			Repo:      r.Repo,
			DocID:     r.DocID,
			Type:      r.Type,
			Title:     r.Title,
			Path:      r.Path,
			Status:    r.Status,
			Author:    r.Author,
			Created:   r.Created,
			UpdatedAt: formatUnix(r.UpdatedAt),
			Snippet:   r.Formatted.Body,
		}
	}
	return hits, nil
}

// formatUnix renders Unix seconds as RFC3339 in UTC, or "" for zero — the
// wire's not-applicable convention. The zone is pinned rather than left to
// the process's own, so a hit's stamp is byte-identical to the
// Document.updated_at the read endpoints serve for the same row
// (DESIGN-0005). The index stores whole seconds and RFC3339 renders whole
// seconds, so nothing is lost in the conversion.
func formatUnix(sec int64) string {
	if sec == 0 {
		return ""
	}
	return time.Unix(sec, 0).UTC().Format(time.RFC3339)
}

// parseFacets decodes Meilisearch's facetDistribution, shaped as
// {"type":{"rfc":2},"status":{"Accepted":1,"Draft":1},...}.
func parseFacets(raw json.RawMessage) (map[string]FacetMap, error) {
	out := make(map[string]FacetMap)
	if len(raw) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode facet distribution: %w", err)
	}
	return out, nil
}

// buildFilter builds the Meilisearch filter expression from p. It always scopes
// to the authorized repos (a nil AllowedRepoIDs disables the scope, e.g. tests;
// an empty slice yields a match-nothing filter) and ANDs any facet filters.
// Returns "" when nothing constrains the query.
func buildFilter(p *SearchParams) string {
	var parts []string

	if p.AllowedRepoIDs != nil {
		if len(p.AllowedRepoIDs) == 0 {
			// No repo is authorized: match nothing (ids are positive serials).
			return "repo_id IN [-1]"
		}
		ids := make([]string, len(p.AllowedRepoIDs))
		for i, id := range p.AllowedRepoIDs {
			ids[i] = strconv.FormatInt(id, 10)
		}
		parts = append(parts, "repo_id IN ["+strings.Join(ids, ", ")+"]")
	}

	parts = appendEq(parts, "repo", p.Repo)
	parts = appendEq(parts, "type", p.Type)
	parts = appendEq(parts, "status", p.Status)
	parts = appendEq(parts, "author", p.Author)
	parts = appendEq(parts, "source", p.Source)

	return strings.Join(parts, " AND ")
}

// appendEq appends a `field = "value"` clause when value is non-empty, escaping
// the value for the Meilisearch filter string.
func appendEq(parts []string, field, value string) []string {
	if value == "" {
		return parts
	}
	return append(parts, field+` = "`+filterValue(value)+`"`)
}

// filterValue escapes a user value for use inside a double-quoted Meilisearch
// filter literal.
func filterValue(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	return v
}
