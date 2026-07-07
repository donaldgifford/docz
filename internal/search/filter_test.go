package search

import (
	"encoding/json"
	"testing"
)

func TestBuildFilter(t *testing.T) {
	tests := []struct {
		name string
		p    SearchParams
		want string
	}{
		{
			name: "nil allowed repos, no facets",
			p:    SearchParams{AllowedRepoIDs: nil},
			want: "",
		},
		{
			name: "nil allowed repos with a facet (authz disabled, e.g. tests)",
			p:    SearchParams{AllowedRepoIDs: nil, Type: "rfc"},
			want: `type = "rfc"`,
		},
		{
			name: "empty allowed set matches nothing",
			p:    SearchParams{AllowedRepoIDs: []int64{}},
			want: "repo_id IN [-1]",
		},
		{
			name: "allowed ids scope the query",
			p:    SearchParams{AllowedRepoIDs: []int64{1, 2}},
			want: "repo_id IN [1, 2]",
		},
		{
			name: "ids AND every facet, in order",
			p: SearchParams{
				AllowedRepoIDs: []int64{7},
				Repo:           "acme/platform", Type: "rfc", Status: "Accepted", Author: "jane",
			},
			want: `repo_id IN [7] AND repo = "acme/platform" AND type = "rfc" AND ` +
				`status = "Accepted" AND author = "jane"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := buildFilter(&tc.p); got != tc.want {
				t.Errorf("buildFilter = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFilterValueEscaping(t *testing.T) {
	// A value with a quote or backslash must be escaped so it can't break out of
	// the double-quoted filter literal (filter injection).
	p := SearchParams{AllowedRepoIDs: nil, Author: `a"b\c`}
	got := buildFilter(&p)
	want := `author = "a\"b\\c"`
	if got != want {
		t.Errorf("buildFilter escaping = %q, want %q", got, want)
	}
}

func TestParseFacets(t *testing.T) {
	t.Run("empty is a non-nil empty map", func(t *testing.T) {
		got, err := parseFacets(nil)
		if err != nil {
			t.Fatalf("parseFacets(nil): %v", err)
		}
		if got == nil || len(got) != 0 {
			t.Errorf("got %v, want an empty non-nil map", got)
		}
	})

	t.Run("valid distribution", func(t *testing.T) {
		raw := json.RawMessage(`{"type":{"rfc":2},"status":{"Accepted":1,"Draft":1}}`)
		got, err := parseFacets(raw)
		if err != nil {
			t.Fatalf("parseFacets: %v", err)
		}
		if got["type"]["rfc"] != 2 || got["status"]["Draft"] != 1 {
			t.Errorf("facets = %v, want type.rfc=2 and status.Draft=1", got)
		}
	})

	t.Run("invalid JSON errors", func(t *testing.T) {
		if _, err := parseFacets(json.RawMessage(`{not json`)); err == nil {
			t.Error("parseFacets on invalid JSON returned nil error")
		}
	})
}
