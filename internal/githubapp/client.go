// Package githubapp authenticates to GitHub as the docz-api App and fetches a
// repo's docz content over the Git Trees API (no checkout). It is the concrete
// implementation of ingest.RepoFetcher.
//
// Authentication uses the App JWT → installation-token flow via
// bradleyfalzon/ghinstallation, which signs a short-lived app JWT, exchanges it
// for an installation access token, and caches/refreshes that token
// transparently on every request.
package githubapp

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v88/github"

	"github.com/donaldgifford/docz-api/internal/ingest"
	doczdoc "github.com/donaldgifford/docz/pkg/doczcore/document"
)

const (
	defaultAPIBase = "https://api.github.com"
	doczConfigFile = ".docz.yaml"
	changelogFile  = "CHANGELOG.md"
)

// Client fetches repo docz content from GitHub as one App installation. It
// satisfies ingest.RepoFetcher.
type Client struct {
	gh *github.Client
}

var _ ingest.RepoFetcher = (*Client)(nil)

// NewClient builds a Client authenticated as the given installation. pemKey is
// the PEM-encoded RSA app private key; apiBase overrides the GitHub API root
// for GitHub Enterprise ("" or the public root uses api.github.com).
func NewClient(appID int64, pemKey []byte, apiBase string, installationID int64) (*Client, error) {
	itr, err := ghinstallation.New(http.DefaultTransport, appID, installationID, pemKey)
	if err != nil {
		return nil, fmt.Errorf("build installation transport: %w", err)
	}

	opts := []github.ClientOptionsFunc{github.WithTransport(itr)}
	if apiBase != "" && apiBase != defaultAPIBase {
		base := strings.TrimSuffix(apiBase, "/")
		itr.BaseURL = base
		opts = append(opts, github.WithEnterpriseURLs(base, base))
	}

	gh, err := github.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("build github client: %w", err)
	}
	return &Client{gh: gh}, nil
}

// Fetch resolves the default-branch HEAD, pulls the recursive tree, and fetches
// .docz.yaml, an optional root CHANGELOG.md, and every doc blob matching the
// docz filename convention. Precise per-type filtering is left to ingest, which
// has the parsed config; githubapp only applies the convention filter.
func (c *Client) Fetch(ctx context.Context, owner, name string) (*ingest.RepoSnapshot, error) {
	repo, _, err := c.gh.Repositories.Get(ctx, owner, name)
	if err != nil {
		return nil, fmt.Errorf("get repo %s/%s: %w", owner, name, err)
	}
	branch := repo.GetDefaultBranch()

	ref, _, err := c.gh.Git.GetRef(ctx, owner, name, "heads/"+branch)
	if err != nil {
		return nil, fmt.Errorf("get ref heads/%s: %w", branch, err)
	}
	headSHA := ref.GetObject().GetSHA()

	tree, _, err := c.gh.Git.GetTree(ctx, owner, name, headSHA, true)
	if err != nil {
		return nil, fmt.Errorf("get tree %s: %w", headSHA, err)
	}
	if tree.GetTruncated() {
		return nil, fmt.Errorf("tree for %s/%s at %s is truncated; shallow-clone path not implemented", owner, name, headSHA)
	}

	configSHA, changelogSHA, docEntries := classifyTree(tree)
	if configSHA == "" {
		return nil, fmt.Errorf("%s/%s has no %s at HEAD", owner, name, doczConfigFile)
	}

	snap := &ingest.RepoSnapshot{HeadSHA: headSHA, DefaultBranch: branch}
	if snap.ConfigYAML, err = c.fetchBlob(ctx, owner, name, configSHA); err != nil {
		return nil, fmt.Errorf("fetch %s: %w", doczConfigFile, err)
	}
	if changelogSHA != "" {
		if snap.ChangelogMD, err = c.fetchBlob(ctx, owner, name, changelogSHA); err != nil {
			return nil, fmt.Errorf("fetch %s: %w", changelogFile, err)
		}
		snap.ChangelogSHA = changelogSHA
	}
	if snap.Blobs, err = c.fetchDocBlobs(ctx, owner, name, docEntries); err != nil {
		return nil, err
	}
	return snap, nil
}

// classifyTree splits a recursive tree into the .docz.yaml sha, the root
// CHANGELOG.md sha (empty if absent), and the doc blobs matching the docz
// filename convention.
func classifyTree(tree *github.Tree) (configSHA, changelogSHA string, docs []*github.TreeEntry) {
	for _, e := range tree.Entries {
		if e.GetType() != "blob" {
			continue
		}
		switch p := e.GetPath(); {
		case p == doczConfigFile:
			configSHA = e.GetSHA()
		case p == changelogFile:
			changelogSHA = e.GetSHA()
		case doczdoc.IsDoczFile(path.Base(p)):
			docs = append(docs, e)
		}
	}
	return configSHA, changelogSHA, docs
}

// fetchDocBlobs fetches every doc blob, preserving repo-relative paths.
func (c *Client) fetchDocBlobs(
	ctx context.Context, owner, name string, entries []*github.TreeEntry,
) ([]ingest.BlobEntry, error) {
	blobs := make([]ingest.BlobEntry, 0, len(entries))
	for _, e := range entries {
		content, err := c.fetchBlob(ctx, owner, name, e.GetSHA())
		if err != nil {
			return nil, fmt.Errorf("fetch blob %s: %w", e.GetPath(), err)
		}
		blobs = append(blobs, ingest.BlobEntry{
			Path:    e.GetPath(),
			GitSHA:  e.GetSHA(),
			Content: content,
		})
	}
	return blobs, nil
}

// fetchBlob fetches one blob by sha and decodes it.
func (c *Client) fetchBlob(ctx context.Context, owner, name, sha string) ([]byte, error) {
	blob, _, err := c.gh.Git.GetBlob(ctx, owner, name, sha)
	if err != nil {
		return nil, fmt.Errorf("get blob %s: %w", sha, err)
	}
	return decodeBlob(blob)
}

// decodeBlob decodes a Git blob's content per its declared encoding. GitHub
// wraps base64 payloads at 76 columns with newlines, which must be stripped.
func decodeBlob(blob *github.Blob) ([]byte, error) {
	switch enc := blob.GetEncoding(); enc {
	case "base64":
		raw := strings.ReplaceAll(blob.GetContent(), "\n", "")
		data, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("decode base64 blob: %w", err)
		}
		return data, nil
	case "utf-8", "":
		return []byte(blob.GetContent()), nil
	default:
		return nil, fmt.Errorf("unsupported blob encoding %q", enc)
	}
}
