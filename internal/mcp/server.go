// Package mcp is docz-api's read-only MCP server (DESIGN-0023, IMPL-0026
// Phase 5): six tools and one resource family over the same store and
// search surfaces the REST API reads, served stateless on the 2026-07-28
// revision at POST /mcp.
//
// Every tool stands on internal/httpapi's exported StoreReader, Searcher,
// ResolveType, and DTO builders, so a tool's structuredContent is the REST
// response's JSON and the two surfaces cannot drift (IMPL-0026 question 6).
// The caller's authorize.FromContext set is applied to every repository
// lookup, which the SDK makes possible by carrying the HTTP request's
// context values through to each handler. Nothing here writes.
package mcp

import (
	"context"
	"log/slog"
	"net/http"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/donaldgifford/docz/v2/internal/httpapi"
)

// ProtocolVersion is the one revision the server speaks (DESIGN-0023 §2,
// question 1): sessionless, with Mcp-Method and Mcp-Name headers and
// cache hints on every list.
const ProtocolVersion = "2026-07-28"

// listTTLMs is the cache hint on server/discover and every list: the tool
// set and the resource templates are fixed at build time, so an hour is
// long enough to matter and short enough that a deploy is picked up.
const listTTLMs = 60 * 60 * 1000

// Deps is what the server reads through. Searcher may be nil, in which
// case search_docs answers "search unavailable". Logger defaults to
// slog.Default.
type Deps struct {
	Store    httpapi.StoreReader
	Searcher httpapi.Searcher
	Version  string
	Logger   *slog.Logger
}

// New builds the SDK server: Implementation{Name: "docz-api"}, the protocol
// narrowed to ProtocolVersion, a one-hour ttlMs on every list, the six
// tools, the resource templates, and the observing middleware that meters,
// traces, and audits each call.
func New(deps *Deps) *sdk.Server {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	srv := sdk.NewServer(
		&sdk.Implementation{Name: "docz-api", Version: deps.Version},
		&sdk.ServerOptions{
			Instructions:              instructions,
			SupportedProtocolVersions: []string{ProtocolVersion},
			SetCacheable:              setCacheable,
		},
	)
	srv.AddReceivingMiddleware(observe(logger))

	t := &tools{store: deps.Store, searcher: deps.Searcher, logger: logger}
	t.register(srv)
	registerResources(srv, t)

	return srv
}

// Handler returns the Streamable HTTP handler for srv in stateless mode:
// every POST is a whole request, GET and DELETE are 405, and no session id
// is read or set (DESIGN-0023 §2).
func Handler(srv *sdk.Server) http.Handler {
	return sdk.NewStreamableHTTPHandler(
		func(*http.Request) *sdk.Server { return srv },
		&sdk.StreamableHTTPOptions{Stateless: true},
	)
}

// setCacheable gives server/discover and the list results a one-hour
// ttlMs. A resources/read result is a document the next ingest may change
// and is scoped to the caller, so it keeps the SDK's default of stale.
func setCacheable(_ context.Context, req sdk.Request, c *sdk.Cacheable) {
	if _, read := req.(*sdk.ReadResourceRequest); read {
		return
	}
	c.TTLMs = listTTLMs
}

// instructions is what a client shows its model about this server.
const instructions = `docz-api serves a fleet's documentation: RFCs, ADRs, design documents,
implementation plans, investigations, and runbooks, one repository at a
time. Start with list_repos, then list_docs for a repository, then get_doc
for one document's markdown and typed fields. search_docs finds documents
across every repository you may read. get_impl_progress answers "how far
along is this plan" without the document's text. Every tool is read-only.`
