package mcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/donaldgifford/docz/v2/internal/authorize"
	"github.com/donaldgifford/docz/v2/internal/httpapi"
	"github.com/donaldgifford/docz/v2/internal/search"
	"github.com/donaldgifford/docz/v2/internal/store"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
)

// The tool names, which are also the metric's tool label and the audit
// line's tool field.
const (
	ToolListRepos       = "list_repos"
	ToolListDocs        = "list_docs"
	ToolGetDoc          = "get_doc"
	ToolSearchDocs      = "search_docs"
	ToolGetImplProgress = "get_impl_progress"
	ToolGetPage         = "get_page"
)

// The errors a tool returns, in the REST API's wording (DESIGN-0023 §3).
// A store error's text never reaches a client: it is logged and the
// caller sees errInternal.
var (
	errRepoNotFound      = errors.New("repo not found")
	errDocNotFound       = errors.New("document not found")
	errTypeNotFound      = errors.New("type not found")
	errPageNotFound      = errors.New("page not found")
	errIndexNotFound     = errors.New("index not found")
	errChangelogNotFound = errors.New("changelog not found")
	errSearchUnavailable = errors.New("search unavailable")
	errInternal          = errors.New("internal error")
)

// The two page paths get_page answers from the repository row rather than
// the pages table.
const (
	pageIndex     = "index"
	pageChangelog = "changelog"
)

// tools holds what every tool reads through.
type tools struct {
	store    httpapi.StoreReader
	searcher httpapi.Searcher
	logger   *slog.Logger
}

// annotations are the same for every tool: read-only, idempotent, and a
// closed world (DESIGN-0023 §3).
func annotations(title string) *sdk.ToolAnnotations {
	closed := false

	return &sdk.ToolAnnotations{
		Title:          title,
		ReadOnlyHint:   true,
		IdempotentHint: true,
		OpenWorldHint:  &closed,
	}
}

// register adds the six tools to srv.
func (t *tools) register(srv *sdk.Server) {
	sdk.AddTool(srv, &sdk.Tool{
		Name:        ToolListRepos,
		Description: "List every repository you may read, with its document types.",
		Annotations: annotations("List repositories"),
	}, t.listRepos)
	sdk.AddTool(srv, &sdk.Tool{
		Name: ToolListDocs,
		Description: "List a repository's documents without their text: every type, " +
			"or one type by name, id prefix, or alias, optionally filtered by status.",
		Annotations: annotations("List documents"),
	}, t.listDocs)
	sdk.AddTool(srv, &sdk.Tool{
		Name: ToolGetDoc,
		Description: "Read one document by its id (IMPL-0018): the summary, the full markdown, " +
			"its Confluence page when exported, and the typed fields its type parses to.",
		Annotations: annotations("Read a document"),
	}, t.getDoc)
	sdk.AddTool(srv, &sdk.Tool{
		Name: ToolSearchDocs,
		Description: "Full-text search across every repository you may read, with snippets and facet counts. " +
			"Filter by repo, type, status, author, or source (doc or page).",
		Annotations: annotations("Search documents"),
	}, t.searchDocs)
	sdk.AddTool(srv, &sdk.Tool{
		Name: ToolGetImplProgress,
		Description: "How far along an implementation plan is: each phase's tasks with done and total counts " +
			"and its open questions, without the document's text.",
		Annotations: annotations("Implementation progress"),
	}, t.getImplProgress)
	sdk.AddTool(srv, &sdk.Tool{
		Name: ToolGetPage,
		Description: "Read a repository's landing page (path \"index\"), its changelog (path \"changelog\"), " +
			"or a published page by its path.",
		Annotations: annotations("Read a page"),
	}, t.getPage)
}

// listDocsIn is list_docs's input.
type listDocsIn struct {
	Repo   string `json:"repo"             jsonschema:"the repository as owner/name"`
	Type   string `json:"type,omitempty"   jsonschema:"a document type by name, id prefix, or alias; every type when omitted"`
	Status string `json:"status,omitempty" jsonschema:"keep only documents with this status"`
}

// docIn is get_doc's and get_impl_progress's input.
type docIn struct {
	Repo string `json:"repo" jsonschema:"the repository as owner/name"`
	ID   string `json:"id"   jsonschema:"the document id from its frontmatter, such as IMPL-0018"`
}

// searchIn is search_docs's input. It carries no sort or offset: an agent
// asks for the best matches, and the REST API is there for paging.
type searchIn struct {
	Query  string `json:"query"            jsonschema:"the search terms"`
	Repo   string `json:"repo,omitempty"   jsonschema:"keep hits from this repository (owner/name)"`
	Type   string `json:"type,omitempty"   jsonschema:"keep hits of this document type"`
	Status string `json:"status,omitempty" jsonschema:"keep hits with this status"`
	Author string `json:"author,omitempty" jsonschema:"keep hits by this author"`
	Source string `json:"source,omitempty" jsonschema:"doc for documents, page for published pages"`
	Limit  int64  `json:"limit,omitempty"  jsonschema:"at most this many hits"`
}

// pageIn is get_page's input.
type pageIn struct {
	Repo string `json:"repo" jsonschema:"the repository as owner/name"`
	Path string `json:"path" jsonschema:"index, changelog, or a published page's path"`
}

// repoEntry is a list_repos entry: GET /repos and GET /repos/{o}/{n}/types
// folded into one.
type repoEntry struct {
	Repo          string            `json:"repo"`
	DefaultBranch string            `json:"default_branch"`
	DocsDir       string            `json:"docs_dir"`
	LastSyncedSHA string            `json:"last_synced_sha"`
	Types         []httpapi.TypeDTO `json:"types"`
}

// listReposOut is list_repos's output.
type listReposOut struct {
	Repos []repoEntry `json:"repos"`
}

// listDocsOut is list_docs's output: the REST list entries.
type listDocsOut struct {
	Docs []httpapi.DocumentDTO `json:"docs"`
}

// docOut is get_doc's output: the REST document plus its typed fields.
type docOut struct {
	Repo          string `json:"repo"`
	DocID         string `json:"doc_id"`
	Type          string `json:"type"`
	Title         string `json:"title"`
	Status        string `json:"status"`
	Author        string `json:"author"`
	Created       string `json:"created"`
	Path          string `json:"path"`
	GitSHA        string `json:"git_sha"`
	ContentHash   string `json:"content_hash"`
	UpdatedAt     string `json:"updated_at"`
	ConfluenceURL string `json:"confluence_url"`
	Markdown      string `json:"markdown"`
	// Fields is the typed parse under a kind (DESIGN-0023 §4): the type
	// package's Doc for a built-in, else generic.
	Fields any `json:"fields"`
}

// pageOut is get_page's output.
type pageOut struct {
	Repo     string `json:"repo"`
	Path     string `json:"path"`
	Title    string `json:"title"`
	Markdown string `json:"markdown"`
	GitSHA   string `json:"git_sha"`
}

// listRepos folds GET /repos and each repository's types into one call.
func (t *tools) listRepos(ctx context.Context, _ *sdk.CallToolRequest, _ struct{}) (*sdk.CallToolResult, listReposOut, error) {
	repos, err := t.store.ListRepos(ctx)
	if err != nil {
		return nil, listReposOut{}, t.internal(ctx, "list repos", err)
	}
	allowed := authorize.FromContext(ctx)

	out := listReposOut{Repos: []repoEntry{}}
	var text strings.Builder
	for i := range repos {
		if !allowed.Contains(repos[i].ID) {
			continue
		}
		types, err := t.store.GetDocTypesForRepo(ctx, repos[i].ID)
		if err != nil {
			return nil, listReposOut{}, t.internal(ctx, "get doc types", err)
		}
		summary := httpapi.ToRepoSummary(&repos[i])
		entry := repoEntry{
			Repo:          summary.Repo,
			DefaultBranch: summary.DefaultBranch,
			DocsDir:       summary.DocsDir,
			LastSyncedSHA: summary.LastSyncedSHA,
			Types:         httpapi.ToTypeDTOs(types),
		}
		out.Repos = append(out.Repos, entry)
		fmt.Fprintf(&text, "%s (types: %s)\n", entry.Repo, typeNames(entry.Types))
	}
	if len(out.Repos) == 0 {
		text.WriteString("no repositories\n")
	}

	return textResult(text.String()), out, nil
}

// listDocs is GET …/types/{type}/docs, across every type when none is named.
func (t *tools) listDocs(ctx context.Context, _ *sdk.CallToolRequest, in listDocsIn) (*sdk.CallToolResult, listDocsOut, error) {
	repo, err := t.repo(ctx, in.Repo)
	if err != nil {
		return nil, listDocsOut{}, err
	}
	types, err := t.store.GetDocTypesForRepo(ctx, repo.ID)
	if err != nil {
		return nil, listDocsOut{}, t.internal(ctx, "get doc types", err)
	}

	names := make([]string, 0, len(types))
	if in.Type == "" {
		for i := range types {
			names = append(names, types[i].Name)
		}
	} else {
		canonical, ok := httpapi.ResolveType(types, in.Type)
		if !ok {
			return nil, listDocsOut{}, errTypeNotFound
		}
		names = append(names, canonical)
	}

	label := httpapi.RepoLabel(&repo)
	out := listDocsOut{Docs: []httpapi.DocumentDTO{}}
	var text strings.Builder
	for _, name := range names {
		docs, err := t.store.ListDocumentsByType(ctx, repo.ID, name)
		if err != nil {
			return nil, listDocsOut{}, t.internal(ctx, "list documents", err)
		}
		for i := range docs {
			dto := httpapi.ToDocumentSummary(label, &docs[i])
			if in.Status != "" && !strings.EqualFold(dto.Status, in.Status) {
				continue
			}
			out.Docs = append(out.Docs, dto)
			fmt.Fprintf(&text, "%s %s [%s]\n", dto.DocID, dto.Title, dto.Status)
		}
	}
	if len(out.Docs) == 0 {
		text.WriteString("no documents\n")
	}

	return textResult(text.String()), out, nil
}

// getDoc is GET …/docs/{doc_id} plus the typed parse (DESIGN-0023 §4).
func (t *tools) getDoc(ctx context.Context, _ *sdk.CallToolRequest, in docIn) (*sdk.CallToolResult, docOut, error) {
	repo, row, err := t.document(ctx, in.Repo, in.ID)
	if err != nil {
		return nil, docOut{}, err
	}
	dto := httpapi.ToDocument(httpapi.RepoLabel(&repo), &row)
	out := docOut{
		Repo:          dto.Repo,
		DocID:         dto.DocID,
		Type:          dto.Type,
		Title:         dto.Title,
		Status:        dto.Status,
		Author:        dto.Author,
		Created:       dto.Created,
		Path:          dto.Path,
		GitSHA:        dto.GitSHA,
		ContentHash:   dto.ContentHash,
		UpdatedAt:     dto.UpdatedAt,
		ConfluenceURL: dto.ConfluenceURL,
		Markdown:      dto.RawMD,
		Fields:        Fields(dto.Type, []byte(dto.RawMD)),
	}

	return textResult(out.Markdown), out, nil
}

// searchDocs is GET /search with the caller's allowed set as the repo
// filter, so a hit never crosses the authorization boundary.
func (t *tools) searchDocs(ctx context.Context, _ *sdk.CallToolRequest, in *searchIn) (*sdk.CallToolResult, search.SearchResult, error) {
	if t.searcher == nil {
		return nil, search.SearchResult{}, errSearchUnavailable
	}
	params := search.SearchParams{
		Query:          in.Query,
		AllowedRepoIDs: allowedOrNone(ctx),
		Repo:           in.Repo,
		Type:           in.Type,
		Status:         in.Status,
		Author:         in.Author,
		Source:         in.Source,
		Limit:          max(in.Limit, 0),
	}
	result, err := t.searcher.Search(ctx, &params)
	if err != nil {
		t.logger.ErrorContext(ctx, "mcp search failed", "err", err)

		return nil, search.SearchResult{}, errSearchUnavailable
	}

	var text strings.Builder
	fmt.Fprintf(&text, "%d hits\n", result.EstimatedTotal)
	for i := range result.Hits {
		h := &result.Hits[i]
		id := h.DocID
		if id == "" {
			id = h.Path
		}
		fmt.Fprintf(&text, "%s %s %s: %s\n", h.Repo, id, h.Title, h.Snippet)
	}

	return textResult(text.String()), result, nil
}

// getPage is GET …/index, …/changelog, or …/pages/{path}.
func (t *tools) getPage(ctx context.Context, _ *sdk.CallToolRequest, in pageIn) (*sdk.CallToolResult, pageOut, error) {
	repo, err := t.repo(ctx, in.Repo)
	if err != nil {
		return nil, pageOut{}, err
	}
	label := httpapi.RepoLabel(&repo)

	var out pageOut
	switch in.Path {
	case pageIndex:
		if !repo.IndexSha.Valid {
			return nil, pageOut{}, errIndexNotFound
		}
		idx := httpapi.ToRepoIndex(&repo)
		out = pageOut{Repo: label, Path: pageIndex, Title: titleOf(idx.IndexMD, "Index"), Markdown: idx.IndexMD, GitSHA: idx.IndexSHA}
	case pageChangelog:
		if !repo.ChangelogSha.Valid {
			return nil, pageOut{}, errChangelogNotFound
		}
		cl := httpapi.ToRepoChangelog(&repo)
		out = pageOut{
			Repo: label, Path: pageChangelog, Title: titleOf(cl.ChangelogMD, "Changelog"),
			Markdown: cl.ChangelogMD, GitSHA: cl.ChangelogSHA,
		}
	default:
		if !httpapi.ValidPagePath(in.Path) {
			return nil, pageOut{}, errPageNotFound
		}
		page, err := t.store.GetRepoPageByPath(ctx, repo.ID, in.Path)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pageOut{}, errPageNotFound
		}
		if err != nil {
			return nil, pageOut{}, t.internal(ctx, "get page", err)
		}
		dto := httpapi.ToPage(label, &page)
		out = pageOut{Repo: label, Path: dto.Path, Title: dto.Title, Markdown: dto.RawMD, GitSHA: dto.GitSHA}
	}

	return textResult(out.Markdown), out, nil
}

// repo loads owner/name and enforces the caller's allowed set: a missing
// repository and one outside the set are the same "repo not found", the
// REST API's existence hiding (DESIGN-0023 §3).
func (t *tools) repo(ctx context.Context, label string) (store.Repo, error) {
	owner, name, ok := strings.Cut(label, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return store.Repo{}, errRepoNotFound
	}
	repo, err := t.store.GetRepo(ctx, owner, name)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Repo{}, errRepoNotFound
	}
	if err != nil {
		return store.Repo{}, t.internal(ctx, "get repo", err)
	}
	if !authorize.FromContext(ctx).Contains(repo.ID) {
		return store.Repo{}, errRepoNotFound
	}

	return repo, nil
}

// document resolves a document id the way repo.Find does: the type from
// the id's prefix over the repository's doc_types, then the row, which
// must be of that type.
func (t *tools) document(ctx context.Context, label, id string) (store.Repo, store.GetDocumentByIDRow, error) {
	repo, err := t.repo(ctx, label)
	if err != nil {
		return store.Repo{}, store.GetDocumentByIDRow{}, err
	}
	prefix, _, ok := strings.Cut(id, "-")
	if !ok || prefix == "" {
		return store.Repo{}, store.GetDocumentByIDRow{}, errTypeNotFound
	}
	types, err := t.store.GetDocTypesForRepo(ctx, repo.ID)
	if err != nil {
		return store.Repo{}, store.GetDocumentByIDRow{}, t.internal(ctx, "get doc types", err)
	}
	canonical, ok := httpapi.ResolveType(types, prefix)
	if !ok {
		return store.Repo{}, store.GetDocumentByIDRow{}, errTypeNotFound
	}
	row, err := t.store.GetDocumentByID(ctx, repo.ID, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.Document.Type != canonical) {
		return store.Repo{}, store.GetDocumentByIDRow{}, errDocNotFound
	}
	if err != nil {
		return store.Repo{}, store.GetDocumentByIDRow{}, t.internal(ctx, "get document", err)
	}

	return repo, row, nil
}

// internal logs a store error and returns the opaque error the caller
// sees: a tool never returns a store error's text.
func (t *tools) internal(ctx context.Context, op string, err error) error {
	t.logger.ErrorContext(ctx, "mcp store error", "op", op, "err", err)

	return errInternal
}

// allowedOrNone is the caller's allowed set for the search filter. A nil
// set would disable the repo scope in the search layer, which is the
// library convenience and never what a request means, so a caller with no
// set matches nothing.
func allowedOrNone(ctx context.Context) []int64 {
	allowed := authorize.FromContext(ctx)
	if allowed == nil {
		return []int64{}
	}

	return allowed
}

// textResult is the one text block beside structuredContent.
func textResult(text string) *sdk.CallToolResult {
	return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: text}}}
}

// typeNames joins the type names for the text block.
func typeNames(types []httpapi.TypeDTO) string {
	names := make([]string, len(types))
	for i := range types {
		names[i] = types[i].Name
	}

	return strings.Join(names, ", ")
}

// titleOf is a page's first H1, else the fallback.
func titleOf(markdown, fallback string) string {
	if title := docparse.Title([]byte(markdown)); title != "" {
		return title
	}

	return fallback
}

// itoa is strconv.Itoa under a name that reads in the text blocks.
func itoa(n int) string { return strconv.Itoa(n) }
