package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/donaldgifford/docz/v2/internal/auth"
	"github.com/donaldgifford/docz/v2/internal/authorize"
	"github.com/donaldgifford/docz/v2/internal/search"
	"github.com/donaldgifford/docz/v2/internal/session"
	"github.com/donaldgifford/docz/v2/internal/telemetry"
)

// principal is the token caller the in-memory tests run as.
func principal() *session.Session {
	return &session.Session{
		ID:       "jti-1",
		Identity: auth.Identity{Provider: "github", Subject: "42", Login: "jane"},
		Via:      session.ViaToken,
		Scopes:   []string{"public:read", "team:read"},
		ClientID: "https://agent.example/client.json",
		Issuer:   "https://docz.example",
	}
}

// harness is a server and a client joined by the in-memory transport,
// the server side's context carrying the principal and its allowed set.
type harness struct {
	cs       *sdk.ClientSession
	searcher *fakeSearcher
	logs     *bytes.Buffer
}

func newHarness(t *testing.T, st *fakeStore, allowed authorize.AllowedRepos, withSearch bool) *harness {
	t.Helper()
	logs := &bytes.Buffer{}
	deps := &Deps{Store: st, Version: "test", Logger: slog.New(slog.NewJSONHandler(logs, nil))}
	var searcher *fakeSearcher
	if withSearch {
		searcher = &fakeSearcher{result: search.SearchResult{
			Query:          "ingest",
			EstimatedTotal: 1,
			Hits: []search.SearchHit{{
				Source: "doc", Repo: "acme/platform", DocID: "RFC-0001", Type: "rfc", Title: "Booty RFC",
				Snippet: "the <em>ingest</em> path",
			}},
			Facets: map[string]search.FacetMap{"type": {"rfc": 1}},
		}}
		deps.Searcher = searcher
	}
	srv := New(deps)

	serverCtx := authorize.ContextWithAllowed(session.ContextWithSession(context.Background(), principal()), allowed)
	ct, stT := sdk.NewInMemoryTransports()
	ss, err := srv.Connect(serverCtx, stT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })

	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	return &harness{cs: cs, searcher: searcher, logs: logs}
}

// call runs one tool and decodes its structuredContent into out.
func (h *harness) call(t *testing.T, tool string, args map[string]any, out any) *sdk.CallToolResult {
	t.Helper()
	res, err := h.cs.CallTool(context.Background(), &sdk.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", tool, err)
	}
	if out != nil && !res.IsError {
		raw, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatalf("decoding %s structuredContent: %v\n%s", tool, err, raw)
		}
	}

	return res
}

// text is the result's one text block.
func text(t *testing.T, res *sdk.CallToolResult) string {
	t.Helper()
	if len(res.Content) != 1 {
		t.Fatalf("content blocks = %d, want 1", len(res.Content))
	}
	tc, ok := res.Content[0].(*sdk.TextContent)
	if !ok {
		t.Fatalf("content[0] is %T, want text", res.Content[0])
	}

	return tc.Text
}

func TestDiscoverAndLists(t *testing.T) {
	t.Parallel()
	h := newHarness(t, seededStore(t), authorize.AllowedRepos{repoPlatform}, true)

	init := h.cs.InitializeResult()
	if init.ServerInfo.Name != "docz-api" || init.ServerInfo.Version != "test" {
		t.Errorf("serverInfo = %+v, want docz-api test", init.ServerInfo)
	}
	if init.ProtocolVersion != ProtocolVersion {
		t.Errorf("protocolVersion = %q, want %q", init.ProtocolVersion, ProtocolVersion)
	}

	tools, err := h.cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if tools.TTLMs != listTTLMs {
		t.Errorf("tools/list ttlMs = %d, want %d", tools.TTLMs, listTTLMs)
	}
	want := []string{ToolGetDoc, ToolGetImplProgress, ToolGetPage, ToolListDocs, ToolListRepos, ToolSearchDocs}
	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
		a := tool.Annotations
		if a == nil || !a.ReadOnlyHint || !a.IdempotentHint || a.OpenWorldHint == nil || *a.OpenWorldHint {
			t.Errorf("%s annotations = %+v, want readOnly, idempotent, closed world", tool.Name, a)
		}
		if tool.OutputSchema == nil {
			t.Errorf("%s has no outputSchema", tool.Name)
		}
	}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("tools = %v, want %v", names, want)
	}

	templates, err := h.cs.ListResourceTemplates(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if templates.TTLMs != listTTLMs {
		t.Errorf("resources/templates/list ttlMs = %d, want %d", templates.TTLMs, listTTLMs)
	}
	if n := len(templates.ResourceTemplates); n != len(builtinTypes)+1 {
		t.Errorf("templates = %d, want %d", n, len(builtinTypes)+1)
	}
}

func TestListRepos(t *testing.T) {
	t.Parallel()
	h := newHarness(t, seededStore(t), authorize.AllowedRepos{repoPlatform}, true)

	var out listReposOut
	res := h.call(t, ToolListRepos, nil, &out)
	if len(out.Repos) != 1 || out.Repos[0].Repo != "acme/platform" {
		t.Fatalf("repos = %+v, want acme/platform only", out.Repos)
	}
	if got := typeNames(out.Repos[0].Types); got != "rfc, impl, frameworks" {
		t.Errorf("types = %q", got)
	}
	if !strings.Contains(text(t, res), "acme/platform (types: rfc, impl, frameworks)") {
		t.Errorf("text = %q", text(t, res))
	}
}

func TestListReposNoneAllowed(t *testing.T) {
	t.Parallel()
	h := newHarness(t, seededStore(t), authorize.AllowedRepos{}, true)

	var out listReposOut
	h.call(t, ToolListRepos, nil, &out)
	if len(out.Repos) != 0 {
		t.Errorf("repos = %+v, want none", out.Repos)
	}
}

func TestListDocs(t *testing.T) {
	t.Parallel()
	h := newHarness(t, seededStore(t), authorize.AllowedRepos{repoPlatform}, true)

	tests := []struct {
		name    string
		args    map[string]any
		wantIDs string
		wantErr string
	}{
		{"every type", map[string]any{"repo": "acme/platform"}, "RFC-0001,IMPL-0007,FW-0001", ""},
		{"by alias", map[string]any{"repo": "acme/platform", "type": "implementation"}, "IMPL-0007", ""},
		{"by prefix folded", map[string]any{"repo": "acme/platform", "type": "fw"}, "FW-0001", ""},
		{"by status", map[string]any{"repo": "acme/platform", "status": "draft"}, "FW-0001", ""},
		{"unknown type", map[string]any{"repo": "acme/platform", "type": "plan"}, "", "type not found"},
		{"hidden repo", map[string]any{"repo": "acme/hidden"}, "", "repo not found"},
		{"missing repo", map[string]any{"repo": "acme/nope"}, "", "repo not found"},
		{"malformed repo", map[string]any{"repo": "platform"}, "", "repo not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var out listDocsOut
			res := h.call(t, ToolListDocs, tt.args, &out)
			if tt.wantErr != "" {
				if !res.IsError || text(t, res) != tt.wantErr {
					t.Fatalf("isError=%v text=%q, want error %q", res.IsError, text(t, res), tt.wantErr)
				}

				return
			}
			if res.IsError {
				t.Fatalf("unexpected error: %s", text(t, res))
			}
			var ids []string
			for _, d := range out.Docs {
				if d.RawMD != "" {
					t.Errorf("%s carries raw_md in a list", d.DocID)
				}
				ids = append(ids, d.DocID)
			}
			if got := strings.Join(ids, ","); got != tt.wantIDs {
				t.Errorf("docs = %s, want %s", got, tt.wantIDs)
			}
		})
	}
}

func TestGetDoc(t *testing.T) {
	t.Parallel()
	h := newHarness(t, seededStore(t), authorize.AllowedRepos{repoPlatform}, true)

	var out struct {
		docOut
		Fields map[string]any `json:"fields"`
	}
	res := h.call(t, ToolGetDoc, map[string]any{"repo": "acme/platform", "id": "IMPL-0007"}, &out)
	if res.IsError {
		t.Fatalf("error: %s", text(t, res))
	}
	if out.DocID != "IMPL-0007" || out.Type != "impl" || !strings.HasPrefix(out.Markdown, "---\nid: IMPL-0007") {
		t.Errorf("doc = %+v", out.docOut)
	}
	if out.Fields["kind"] != KindImpl {
		t.Errorf("fields.kind = %v, want impl", out.Fields["kind"])
	}
	if phases, _ := out.Fields["Phases"].([]any); len(phases) == 0 {
		t.Errorf("fields carries no phases: %v", out.Fields)
	}
	if text(t, res) != out.Markdown {
		t.Error("text block is not the markdown")
	}

	var rfcOut struct {
		ConfluenceURL string `json:"confluence_url"`
		Fields        struct {
			Kind string `json:"kind"`
		} `json:"fields"`
	}
	h.call(t, ToolGetDoc, map[string]any{"repo": "acme/platform", "id": "RFC-0001"}, &rfcOut)
	if rfcOut.Fields.Kind != KindRFC || rfcOut.ConfluenceURL == "" {
		t.Errorf("rfc = %+v", rfcOut)
	}

	var fw struct {
		Fields GenericFields `json:"fields"`
	}
	h.call(t, ToolGetDoc, map[string]any{"repo": "acme/platform", "id": "FW-0001"}, &fw)
	if fw.Fields.Kind != KindGeneric || len(fw.Fields.Headings) != 1 || fw.Fields.ParseError != "" {
		t.Errorf("custom type fields = %+v, want generic with one heading", fw.Fields)
	}

	for name, args := range map[string]map[string]any{
		"wrong type for id": {"repo": "acme/platform", "id": "RFC-0007"},
		"unknown prefix":    {"repo": "acme/platform", "id": "PLAN-0001"},
		"no prefix":         {"repo": "acme/platform", "id": "0001"},
		"hidden repo":       {"repo": "acme/hidden", "id": "RFC-0001"},
	} {
		res := h.call(t, ToolGetDoc, args, nil)
		if !res.IsError {
			t.Errorf("%s: want an error", name)
		}
	}
	if res := h.call(t, ToolGetDoc, map[string]any{"repo": "acme/platform", "id": "RFC-0007"}, nil); text(t, res) != "document not found" {
		t.Errorf("wrong type: %q", text(t, res))
	}
	if res := h.call(t, ToolGetDoc, map[string]any{"repo": "acme/platform", "id": "PLAN-0001"}, nil); text(t, res) != "type not found" {
		t.Errorf("unknown prefix: %q", text(t, res))
	}
}

func TestSearchDocs(t *testing.T) {
	t.Parallel()
	h := newHarness(t, seededStore(t), authorize.AllowedRepos{repoPlatform}, true)

	var out search.SearchResult
	res := h.call(t, ToolSearchDocs, map[string]any{"query": "ingest", "type": "rfc", "limit": 5}, &out)
	if res.IsError {
		t.Fatalf("error: %s", text(t, res))
	}
	got := h.searcher.got
	if got.Query != "ingest" || got.Type != "rfc" || got.Limit != 5 || len(got.AllowedRepoIDs) != 1 || got.AllowedRepoIDs[0] != repoPlatform {
		t.Errorf("params = %+v", got)
	}
	if len(out.Hits) != 1 || out.Hits[0].DocID != "RFC-0001" || out.Facets["type"]["rfc"] != 1 {
		t.Errorf("result = %+v", out)
	}
	if !strings.Contains(text(t, res), "acme/platform RFC-0001 Booty RFC") {
		t.Errorf("text = %q", text(t, res))
	}

	h.searcher.err = context.DeadlineExceeded
	if res := h.call(t, ToolSearchDocs, map[string]any{"query": "x"}, nil); !res.IsError || text(t, res) != "search unavailable" {
		t.Errorf("failed search: isError=%v %q", res.IsError, text(t, res))
	}
	if strings.Contains(h.logs.String(), `"query"`) {
		t.Error("a log line carries the query")
	}

	none := newHarness(t, seededStore(t), authorize.AllowedRepos{repoPlatform}, false)
	if res := none.call(t, ToolSearchDocs, map[string]any{"query": "x"}, nil); !res.IsError || text(t, res) != "search unavailable" {
		t.Errorf("no searcher: isError=%v %q", res.IsError, text(t, res))
	}
}

func TestSearchDocsEmptyAllowedMatchesNothing(t *testing.T) {
	t.Parallel()
	h := newHarness(t, seededStore(t), nil, true)

	h.call(t, ToolSearchDocs, map[string]any{"query": "ingest"}, nil)
	if h.searcher.got.AllowedRepoIDs == nil {
		t.Error("a caller with no allowed set reached the searcher with the nil (unscoped) filter")
	}
}

func TestGetImplProgress(t *testing.T) {
	t.Parallel()
	h := newHarness(t, seededStore(t), authorize.AllowedRepos{repoPlatform}, true)

	var out progressOut
	res := h.call(t, ToolGetImplProgress, map[string]any{"repo": "acme/platform", "id": "IMPL-0007"}, &out)
	if res.IsError {
		t.Fatalf("error: %s", text(t, res))
	}
	if out.DocID != "IMPL-0007" || out.Objective == "" || len(out.Phases) == 0 {
		t.Fatalf("progress = %+v", out)
	}
	done, total := 0, 0
	for _, p := range out.Phases {
		if p.Done > p.Total {
			t.Errorf("phase %s: done %d > total %d", p.Token, p.Done, p.Total)
		}
		done += p.Done
		total += p.Total
		for _, task := range p.Tasks {
			if task.ID == "" {
				t.Errorf("phase %s has a task with no id: %+v", p.Token, task)
			}
		}
	}
	if done != out.Done || total != out.Total || total == 0 {
		t.Errorf("counts: phases sum to %d/%d, document says %d/%d", done, total, out.Done, out.Total)
	}
	if !strings.Contains(text(t, res), "tasks done") {
		t.Errorf("text = %q", text(t, res))
	}

	if res := h.call(t, ToolGetImplProgress, map[string]any{"repo": "acme/platform", "id": "RFC-0001"}, nil); !res.IsError {
		t.Error("an RFC parsed as a plan")
	}
}

func TestGetPage(t *testing.T) {
	t.Parallel()
	h := newHarness(t, seededStore(t), authorize.AllowedRepos{repoPlatform}, true)

	tests := []struct {
		path      string
		wantTitle string
		wantSHA   string
		wantErr   string
	}{
		{"index", "Platform docs", "idxsha", ""},
		{"changelog", "Changelog", "clsha", ""},
		{"guides/setup", "Setup", "pagesha", ""},
		{"guides/nope", "", "", "page not found"},
		{"../etc/passwd", "", "", "page not found"},
		{"", "", "", "page not found"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			var out pageOut
			res := h.call(t, ToolGetPage, map[string]any{"repo": "acme/platform", "path": tt.path}, &out)
			if tt.wantErr != "" {
				if !res.IsError || text(t, res) != tt.wantErr {
					t.Fatalf("isError=%v text=%q, want %q", res.IsError, text(t, res), tt.wantErr)
				}

				return
			}
			if res.IsError {
				t.Fatalf("error: %s", text(t, res))
			}
			if out.Title != tt.wantTitle || out.GitSHA != tt.wantSHA || out.Markdown == "" {
				t.Errorf("page = %+v", out)
			}
		})
	}

	bare := seededStore(t)
	bare.repos[0].IndexSha.Valid = false
	bare.repos[0].ChangelogSha.Valid = false
	hb := newHarness(t, bare, authorize.AllowedRepos{repoPlatform}, true)
	if res := hb.call(t, ToolGetPage, map[string]any{"repo": "acme/platform", "path": "index"}, nil); text(t, res) != "index not found" {
		t.Errorf("no index: %q", text(t, res))
	}
	if res := hb.call(t, ToolGetPage, map[string]any{"repo": "acme/platform", "path": "changelog"}, nil); text(t, res) != "changelog not found" {
		t.Errorf("no changelog: %q", text(t, res))
	}
}

func TestResources(t *testing.T) {
	t.Parallel()
	h := newHarness(t, seededStore(t), authorize.AllowedRepos{repoPlatform}, true)

	res, err := h.cs.ReadResource(context.Background(), &sdk.ReadResourceParams{URI: "docz://acme/platform/rfc/RFC-0001"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Contents) != 2 || res.Contents[0].MIMEType != mimeMarkdown || res.Contents[1].MIMEType != mimeJSON {
		t.Fatalf("contents = %+v", res.Contents)
	}
	if !strings.Contains(res.Contents[0].Text, "id: RFC-0001") {
		t.Error("markdown content is not the document")
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(res.Contents[1].Text), &fields); err != nil || fields["kind"] != KindRFC {
		t.Errorf("json content = %v (%v)", fields, err)
	}
	if res.TTLMs != 0 {
		t.Errorf("resources/read ttlMs = %d, want stale", res.TTLMs)
	}

	// A token resolving to the type reads the same document; the wrong type
	// does not.
	if _, err := h.cs.ReadResource(context.Background(), &sdk.ReadResourceParams{URI: "docz://acme/platform/implementation/IMPL-0007"}); err != nil {
		t.Errorf("alias type token: %v", err)
	}
	for _, uri := range []string{
		"docz://acme/platform/impl/RFC-0001",
		"docz://acme/hidden/rfc/RFC-0001",
		"docz://acme/platform/rfc/RFC-0009",
		"docz://acme/platform/rfc",
		"https://acme/platform/rfc/RFC-0001",
	} {
		if _, err := h.cs.ReadResource(context.Background(), &sdk.ReadResourceParams{URI: uri}); err == nil {
			t.Errorf("%s: want not found", uri)
		}
	}
}

func TestAuditLineAndMetric(t *testing.T) {
	t.Parallel()
	h := newHarness(t, seededStore(t), authorize.AllowedRepos{repoPlatform}, true)

	h.call(t, ToolGetDoc, map[string]any{"repo": "acme/platform", "id": "RFC-0001"}, nil)
	h.call(t, ToolSearchDocs, map[string]any{"query": "secret words"}, nil)
	if _, err := h.cs.ReadResource(context.Background(), &sdk.ReadResourceParams{URI: "docz://acme/platform/rfc/RFC-0001"}); err != nil {
		t.Fatal(err)
	}

	var lines []map[string]any
	for _, raw := range bytes.Split(bytes.TrimSpace(h.logs.Bytes()), []byte("\n")) {
		var line map[string]any
		if err := json.Unmarshal(raw, &line); err != nil {
			t.Fatalf("log line %q: %v", raw, err)
		}
		if line["msg"] == "mcp call" {
			lines = append(lines, line)
		}
	}
	if len(lines) != 3 {
		t.Fatalf("mcp call lines = %d, want 3:\n%s", len(lines), h.logs.String())
	}
	first := lines[0]
	for key, want := range map[string]any{
		"principal": "user:github:42", "via": "token", "client_id": "https://agent.example/client.json",
		"issuer": "https://docz.example", "tool": ToolGetDoc, "repo": "acme/platform", "doc_id": "RFC-0001", "status": statusOK,
	} {
		if first[key] != want {
			t.Errorf("mcp call %s = %v, want %v", key, first[key], want)
		}
	}
	if _, ok := first["duration"]; !ok {
		t.Error("mcp call line has no duration")
	}
	if scopes, _ := first["scopes"].([]any); len(scopes) != 2 {
		t.Errorf("scopes = %v", first["scopes"])
	}
	if lines[1]["tool"] != ToolSearchDocs || strings.Contains(h.logs.String(), "secret words") {
		t.Errorf("search line = %v; the query must never be logged", lines[1])
	}
	if lines[2]["tool"] != methodReadResource || lines[2]["doc_id"] != "RFC-0001" {
		t.Errorf("resource line = %v", lines[2])
	}

	rec := httptest.NewRecorder()
	telemetry.MetricsHandler().ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", http.NoBody))
	body := rec.Body.String()
	for _, series := range []string{
		`docz_api_mcp_calls_total{status="ok",tool="get_doc"}`,
		`docz_api_mcp_calls_total{status="ok",tool="search_docs"}`,
		`docz_api_mcp_call_duration_seconds_count{tool="get_doc"}`,
	} {
		if !strings.Contains(body, series) {
			t.Errorf("metrics lack %s", series)
		}
	}
}
