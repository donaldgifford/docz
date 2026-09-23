package search

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/meilisearch/meilisearch-go"
)

func TestFormatUnix(t *testing.T) {
	tests := []struct {
		name string
		sec  int64
		want string
	}{
		// Zero is the wire's not-applicable convention, not the epoch: a
		// record with no stamp serves "" like every other unset string field.
		{"zero renders empty", 0, ""},
		{"a stamp renders RFC3339 in UTC", 1750615451, "2025-06-22T18:04:11Z"},
		{"one second past the epoch is still a stamp", 1, "1970-01-01T00:00:01Z"},
		{"a negative stamp still renders", -1, "1969-12-31T23:59:59Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatUnix(tt.sec); got != tt.want {
				t.Errorf("formatUnix(%d) = %q, want %q", tt.sec, got, tt.want)
			}
		})
	}
}

// hitsFromJSON builds a meilisearch.Hits from a raw response fragment, so the
// decode path is exercised through the same JSON tags a live Meilisearch
// response carries.
func hitsFromJSON(t *testing.T, raw string) meilisearch.Hits {
	t.Helper()
	var hits meilisearch.Hits
	if err := json.Unmarshal([]byte(raw), &hits); err != nil {
		t.Fatalf("build hits from %q: %v", raw, err)
	}
	return hits
}

// TestDecodeHitsDates pins the two dated fields end to end: created passes
// through as the stored YYYY-MM-DD string, and updated_at converts from the
// index's Unix seconds to an RFC3339 stamp in UTC. A page hit carries a real
// stamp and an empty created — the shape toIndexPage produces.
func TestDecodeHitsDates(t *testing.T) {
	hits := hitsFromJSON(t, `[
	  {"source":"doc","repo":"acme/platform","doc_id":"RFC-0001","type":"rfc",
	   "title":"Structured logging","path":"docs/rfc/0001-structured-logging.md",
	   "status":"Accepted","author":"Jane Dev","created":"2026-01-15",
	   "updated_at":1750615451,"body":"full body",
	   "_formatted":{"body":"…structured <em>logging</em>…"}},
	  {"source":"page","repo":"acme/platform","title":"Setup Guide",
	   "path":"guides/setup.md","created":"","updated_at":1750615456,
	   "body":"walkthrough","_formatted":{"body":"a <em>walkthrough</em>"}}
	]`)

	got, err := decodeHits(hits)
	if err != nil {
		t.Fatalf("decodeHits: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("decoded %d hits, want 2", len(got))
	}

	doc := got[0]
	if doc.Created != "2026-01-15" {
		t.Errorf("doc created = %q, want 2026-01-15 (passed through, not converted)", doc.Created)
	}
	if doc.UpdatedAt != "2025-06-22T18:04:11Z" {
		t.Errorf("doc updated_at = %q, want 2025-06-22T18:04:11Z", doc.UpdatedAt)
	}
	if doc.Snippet != "…structured <em>logging</em>…" {
		t.Errorf("doc snippet = %q, want the _formatted body", doc.Snippet)
	}

	page := got[1]
	if page.Created != "" {
		t.Errorf("page created = %q, want empty (pages carry no authored date)", page.Created)
	}
	if page.UpdatedAt != "2025-06-22T18:04:16Z" {
		t.Errorf("page updated_at = %q, want a real stamp — repo_pages stamps every row", page.UpdatedAt)
	}
}

// TestDecodeHitsMissingDates covers the attribute-absent case: a hit that
// carries neither date decodes to the "" convention rather than an epoch
// stamp. This is the shape a stale retrieve list would produce, so the
// assertion also documents why the retrieve list is the first drop site.
func TestDecodeHitsMissingDates(t *testing.T) {
	hits := hitsFromJSON(t, `[{"source":"doc","repo":"acme/platform","doc_id":"RFC-0001",
	  "title":"No dates","_formatted":{"body":"body"}}]`)

	got, err := decodeHits(hits)
	if err != nil {
		t.Fatalf("decodeHits: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("decoded %d hits, want 1", len(got))
	}
	if got[0].Created != "" || got[0].UpdatedAt != "" {
		t.Errorf("created/updated_at = %q/%q, want both empty",
			got[0].Created, got[0].UpdatedAt)
	}
}

func TestParseSort(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"empty is the unsorted default", "", "", false},
		{"updated_at descending", SortUpdatedDesc, SortUpdatedDesc, false},
		{"updated_at ascending", SortUpdatedAsc, SortUpdatedAsc, false},
		{"created descending", SortCreatedDesc, SortCreatedDesc, false},
		{"created ascending", SortCreatedAsc, SortCreatedAsc, false},
		// Direction is part of the token: a bare attribute would need a
		// documented default direction, which the enum exists to avoid.
		{"bare attribute", "updated_at", "", true},
		{"unknown direction", "updated_at:down", "", true},
		{"wrong case", "UPDATED_AT:desc", "", true},
		// Not a sortable attribute — sorting by it would fail in Meilisearch.
		{"unsortable attribute", "body:desc", "", true},
		{"filterable but unsortable attribute", "status:desc", "", true},
		// Whitespace is not trimmed: the enum is the contract, and quietly
		// accepting near-misses invites more leniency questions.
		{"leading whitespace", " updated_at:desc", "", true},
		{"trailing whitespace", "updated_at:desc ", "", true},
		{"a comma list is one token, not two", "updated_at:desc,created:desc", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSort(tt.in)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidSort) {
					t.Fatalf("ParseSort(%q) error = %v, want ErrInvalidSort", tt.in, err)
				}
				// The rejected value belongs in the message, for the caller
				// reading a log line rather than the response body.
				if !strings.Contains(err.Error(), tt.in) {
					t.Errorf("ParseSort(%q) error = %q, want it to quote the input", tt.in, err)
				}
			} else if err != nil {
				t.Fatalf("ParseSort(%q) unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseSort(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestSortKeys pins the secondary key. Every record a reconcile touches
// shares one transaction timestamp, so an updated_at sort ties across a whole
// repository; the secondary resolves those ties by the other date in the same
// direction rather than by Meilisearch's internal order.
func TestSortKeys(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  []string
	}{
		{
			name:  "newest updated falls through to newest created",
			token: SortUpdatedDesc,
			want:  []string{SortUpdatedDesc, SortCreatedDesc},
		},
		{
			name:  "oldest updated falls through to oldest created",
			token: SortUpdatedAsc,
			want:  []string{SortUpdatedAsc, SortCreatedAsc},
		},
		{
			name:  "newest created falls through to newest updated",
			token: SortCreatedDesc,
			want:  []string{SortCreatedDesc, SortUpdatedDesc},
		},
		{
			name:  "oldest created falls through to oldest updated",
			token: SortCreatedAsc,
			want:  []string{SortCreatedAsc, SortUpdatedAsc},
		},
		// Unreachable through the handler, which rejects the token first.
		{name: "unvalidated token yields no sort", token: "nonsense", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortKeys(tt.token)
			if !slices.Equal(got, tt.want) {
				t.Errorf("sortKeys(%q) = %v, want %v", tt.token, got, tt.want)
			}
		})
	}
}

// TestSortTokensCoverSortableAttributes ties the three lists together: every
// accepted token names an attribute the index can actually sort by, and every
// sortable attribute is reachable in both directions. Adding a sortable
// attribute without its tokens (or the reverse) fails here rather than as a
// Meilisearch error on a live query.
func TestSortTokensCoverSortableAttributes(t *testing.T) {
	seen := make(map[string][]string, len(sortableAttributes))
	for token, secondary := range sortSecondary {
		attr, dir, ok := strings.Cut(token, ":")
		if !ok {
			t.Errorf("sort token %q is not <attribute>:<direction>", token)
			continue
		}
		if !slices.Contains(sortableAttributes, attr) {
			t.Errorf("sort token %q names %q, which is not a sortable attribute %v",
				token, attr, sortableAttributes)
		}
		if _, ok := sortSecondary[secondary]; !ok {
			t.Errorf("sort token %q has secondary %q, which is not itself an accepted token",
				token, secondary)
		}
		// The secondary must order the same way, or "newest first" would
		// break ties with the oldest.
		if _, secDir, _ := strings.Cut(secondary, ":"); secDir != dir {
			t.Errorf("sort token %q has secondary %q in direction %q, want %q",
				token, secondary, secDir, dir)
		}
		seen[attr] = append(seen[attr], dir)
	}

	for _, attr := range sortableAttributes {
		dirs := seen[attr]
		slices.Sort(dirs)
		if !slices.Equal(dirs, []string{"asc", "desc"}) {
			t.Errorf("sortable attribute %q has tokens for %v, want both asc and desc", attr, dirs)
		}
	}
}

// TestRankingRulesSortLeads guards the ranking-rule placement. At
// Meilisearch's default position the sort rule only breaks ties within
// relevance, so a "newest first" search would not return the newest hit
// first. The rule is inert on searches that pass no sort, which is what makes
// leading with it safe (DESIGN-0005).
func TestRankingRulesSortLeads(t *testing.T) {
	if len(rankingRules) == 0 || rankingRules[0] != "sort" {
		t.Fatalf("ranking rules = %v, want \"sort\" first", rankingRules)
	}
	// The relevance rules must all still be present behind it: dropping one
	// would silently change unsorted ranking, which this change must not do.
	for _, rule := range []string{"words", "typo", "proximity", "attribute", "exactness"} {
		if !slices.Contains(rankingRules, rule) {
			t.Errorf("ranking rules = %v, want them to retain %q", rankingRules, rule)
		}
	}
}

// TestBuildSearchRequestOmitsOptionalKeys pins the promise that adding sort
// and source left every existing caller's request untouched. Meilisearch
// rejects an empty filter string outright, and a non-nil Sort would wake the
// sort ranking rule that now leads the list — so both keys must be absent,
// not merely empty, when the caller asks for neither.
func TestBuildSearchRequestOmitsOptionalKeys(t *testing.T) {
	req := buildSearchRequest(&SearchParams{Query: "logging"})

	if req.Sort != nil {
		t.Errorf("Sort = %v, want nil on an unsorted search", req.Sort)
	}
	if req.Filter != nil {
		t.Errorf("Filter = %v, want nil when no filter applies", req.Filter)
	}
	if req.Limit != defaultSearchLimit {
		t.Errorf("Limit = %d, want the default %d", req.Limit, defaultSearchLimit)
	}
}

// TestBuildSearchRequestSetsOptionalKeys is the other half: a caller that asks
// for a sort gets both keys, the requested one first.
func TestBuildSearchRequestSetsOptionalKeys(t *testing.T) {
	req := buildSearchRequest(&SearchParams{
		Query:          "logging",
		Sort:           SortUpdatedDesc,
		Source:         "doc",
		AllowedRepoIDs: []int64{7},
		Limit:          5,
	})

	if want := []string{SortUpdatedDesc, SortCreatedDesc}; !slices.Equal(req.Sort, want) {
		t.Errorf("Sort = %v, want %v", req.Sort, want)
	}
	filter, ok := req.Filter.(string)
	if !ok {
		t.Fatalf("Filter = %#v, want a string", req.Filter)
	}
	if !strings.Contains(filter, `source = "doc"`) {
		t.Errorf("Filter = %q, want it to carry the source clause", filter)
	}
	if req.Limit != 5 {
		t.Errorf("Limit = %d, want the caller's 5", req.Limit)
	}
}

// TestSearchRetrievesDatedAttributes guards the retrieve list itself. A hit
// field can only arrive if Search asks Meilisearch for its attribute, and
// that list is invisible to every test that fakes the searcher — the drop
// site INV-0009 F2 found. Keep this in sync with SearchHit.
func TestSearchRetrievesDatedAttributes(t *testing.T) {
	want := []string{
		"source", "repo", "doc_id", "type", "title", "path",
		"status", "author", "created", "updated_at", "body",
	}
	got := retrieveAttributes
	if len(got) != len(want) {
		t.Fatalf("retrieve list = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("retrieve list[%d] = %q, want %q (full list %v)", i, got[i], want[i], got)
		}
	}
}
