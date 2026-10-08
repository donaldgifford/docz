package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/donaldgifford/docz/v2/internal/config"
	"github.com/donaldgifford/docz/v2/pkg/export/confluence"
)

// newConfluenceClient is the one Confluence client the process uses
// (IMPL-0024 Open Question 6): it resolves the cloud id once and shares its
// connections across every export. hc is nil outside tests.
func newConfluenceClient(cfg *config.ConfluenceConfig, hc *http.Client) *confluence.HTTPClient {
	var opts []confluence.HTTPOption
	if hc != nil {
		opts = append(opts, confluence.WithHTTPClient(hc))
	}

	return confluence.NewHTTPClient(cfg.Site, cfg.Email, cfg.APIToken.Reveal(), opts...)
}

// setupConfluence builds the process's Confluence client and checks its
// credential, or returns nil with the export off: no token, no client, and
// no export is ever enqueued.
func setupConfluence(ctx context.Context, cfg *config.ConfluenceConfig) (*confluence.HTTPClient, error) {
	if !cfg.Enabled() {
		return nil, nil //nolint:nilnil // no client is the off state, not an error
	}

	client := newConfluenceClient(cfg, nil)
	if err := checkConfluenceCredentials(ctx, client, cfg.Spaces); err != nil {
		return nil, err
	}

	return client, nil
}

// confluenceCheckTimeout bounds the startup credential check, the same
// budget as the GitHub App's.
const confluenceCheckTimeout = githubCheckTimeout

// checkConfluenceCredentials looks up the allowed spaces at startup, so a
// token Atlassian rejects fails the deploy instead of every export
// (DESIGN-0021 §6). Only a 401 is fatal: a 403, a space that is not found,
// or Confluence being unreachable is logged and the server starts, because
// a Confluence problem must not take down the read API.
func checkConfluenceCredentials(ctx context.Context, client *confluence.HTTPClient, spaces []string) error {
	ctx, cancel := context.WithTimeout(ctx, confluenceCheckTimeout)
	defer cancel()

	found, err := client.Spaces(ctx, spaces)

	var auth *confluence.AuthError

	switch {
	case errors.As(err, &auth) && auth.Status == http.StatusUnauthorized:
		return fmt.Errorf("checking confluence credentials: %w", err)
	case err != nil:
		slog.Warn("could not verify confluence credentials at startup; continuing", "err", err)

		return nil
	}

	for _, key := range spaces {
		if found[key] == "" {
			slog.Warn("an allowed confluence space was not found; exports to it will fail",
				"space", key)
		}
	}

	slog.Info("confluence credentials verified", "spaces", len(found))

	return nil
}
