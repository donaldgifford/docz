package main

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/donaldgifford/docz/v2/internal/config"
	"github.com/donaldgifford/docz/v2/internal/httpapi"
	"github.com/donaldgifford/docz/v2/internal/mcp"
	"github.com/donaldgifford/docz/v2/internal/search"
	"github.com/donaldgifford/docz/v2/internal/store"
)

// mountMCP mounts the MCP server at /mcp when MCP_ENABLED (DESIGN-0023 §2):
// every method reaches the SDK's stateless handler through the gate, so an
// unauthenticated POST is the 401 challenge before the SDK sees it and GET
// and DELETE are the SDK's 405, and the per-principal limiter sits between
// the two. It returns the gate /api/v1 should use: with MCP on, the same
// limiter inside it for token callers; off, gate unchanged, and /mcp stays
// the router's 404.
func mountMCP(
	router chi.Router, cfg *config.Config, st *store.Store, searchClient *search.Client,
	gate func(http.Handler) http.Handler,
) func(http.Handler) http.Handler {
	if !cfg.MCP.Enabled {
		return gate
	}

	// A typed nil must not become a non-nil Searcher interface.
	var searcher httpapi.Searcher
	if searchClient != nil {
		searcher = searchClient
	}
	limiter := mcp.NewLimiter(cfg.MCP.RateLimit)
	srv := mcp.New(&mcp.Deps{Store: st, Searcher: searcher, Version: version})
	router.With(gate, limiter.MCP).Handle("/mcp", mcp.Handler(srv))
	slog.Info("mcp server mounted", "protocol", mcp.ProtocolVersion, "rate_limit_per_minute", cfg.MCP.RateLimit)

	return func(next http.Handler) http.Handler { return gate(limiter.API(next)) }
}
