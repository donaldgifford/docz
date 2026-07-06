package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/donaldgifford/docz-api/internal/authorize"
	"github.com/donaldgifford/docz-api/internal/search"
)

// updateGolden regenerates the contract fixtures instead of asserting them:
//
//	go test ./internal/httpapi -run TestContractGolden -update
//
// The committed fixtures under testdata/contract/ are the frozen wire contract
// (DESIGN-0001 response shapes, consumed cross-repo per DESIGN-0009): any change
// to a field name, JSON type, envelope key, status code, or content-type shows
// up as a golden diff here so a breaking change fails in CI before it ships.
var updateGolden = flag.Bool("update", false, "update contract golden fixtures")

// contractSearcher returns one fixed result so the /search contract is
// deterministic (the real Meilisearch response is exercised by the search
// integration test).
type contractSearcher struct{}

func (contractSearcher) Search(context.Context, *search.SearchParams) (search.SearchResult, error) {
	return search.SearchResult{
		Query:          "intro",
		EstimatedTotal: 1,
		Hits: []search.SearchHit{{
			Repo: "acme/platform", DocID: "FW-0001", Type: "frameworks",
			Title: "Intro", Status: "Draft", Author: "Jane",
			Snippet: "an <em>intro</em> to frameworks",
		}},
		Facets: map[string]search.FacetMap{
			"type":   {"frameworks": 1},
			"status": {"Draft": 1},
		},
	}, nil
}

// contractServer wires the read + search routes exactly as main does, behind the
// authorize seam, so the goldens capture what a real client receives.
func contractServer(st storeReader, authz authorize.Authorizer) http.Handler {
	r := chi.NewRouter()
	NewHandlerWithSearch(st, contractSearcher{}).Mount(r, authorize.Middleware(authz))
	return r
}

// capturedResponse is the golden envelope: it locks the status code and
// Content-Type alongside the JSON body, so a contract regression in any of the
// three is caught.
type capturedResponse struct {
	Status      int             `json:"status"`
	ContentType string          `json:"content_type"`
	Body        json.RawMessage `json:"body"`
}

func TestContractGolden(t *testing.T) {
	st := seededStore()
	srv := contractServer(st, authorize.NewAllReposAuthorizer(st))

	cases := []struct {
		name string
		path string
	}{
		{"list_repos", "/api/v1/repos"},
		{"repo_detail", "/api/v1/repos/acme/platform"},
		{"list_types", "/api/v1/repos/acme/platform/types"},
		{"list_docs", "/api/v1/repos/acme/platform/types/frameworks/docs"},
		{"get_doc", "/api/v1/repos/acme/platform/types/FW/docs/FW-0001"},
		{"search", "/api/v1/search?q=intro"},
		{"error_not_found", "/api/v1/repos/acme/missing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doGet(t, srv, tc.path)

			captured := capturedResponse{
				Status:      rec.Code,
				ContentType: rec.Header().Get("Content-Type"),
				Body:        json.RawMessage(rec.Body.Bytes()),
			}
			got := indentJSON(t, captured)

			golden := filepath.Join("testdata", "contract", tc.name+".json")
			if *updateGolden {
				writeGolden(t, golden, got)
			}
			want := readGolden(t, golden)
			if !bytes.Equal(got, want) {
				t.Errorf(
					"contract drift for %s (GET %s):\n--- got ---\n%s\n--- want ---\n%s\n"+
						"if this change is intentional, run: go test ./internal/httpapi -run TestContractGolden -update",
					tc.name, tc.path, got, want,
				)
			}
		})
	}
}

// indentJSON marshals v and pretty-prints it (which also compacts the embedded
// RawMessage body — stripping any trailing newline from the error encoder — so
// the fixtures are stable and readable).
func indentJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal captured response: %v", err)
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		t.Fatalf("indent captured response: %v", err)
	}
	buf.WriteByte('\n')
	return buf.Bytes()
}

func writeGolden(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir golden dir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write golden %s: %v", path, err)
	}
}

func readGolden(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (run with -update to create it): %v", path, err)
	}
	return data
}
