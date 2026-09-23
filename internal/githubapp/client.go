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
	"gopkg.in/yaml.v3"

	"github.com/donaldgifford/docz-api/internal/ingest"
	doczcfg "github.com/donaldgifford/docz/pkg/doczcore/config"
	doczdoc "github.com/donaldgifford/docz/pkg/doczcore/document"
)

const (
	defaultAPIBase = "https://api.github.com"
	doczConfigFile = ".docz.yaml"
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
// .docz.yaml, the configured changelog and landing-page files when present,
// every doc blob matching the docz filename convention, and — when the api:
// block is enabled — every .md under docs_dir plus each present
// additional_docs file (DESIGN-0004). Precise filtering (per-type assignment,
// api exclusions, page classification) is left to ingest, which has the parsed
// config; githubapp only decides what to fetch.
func (c *Client) Fetch(ctx context.Context, owner, name string) (*ingest.RepoSnapshot, error) {
	branch, headSHA, tree, err := c.resolveHead(ctx, owner, name)
	if err != nil {
		return nil, err
	}

	configSHA := findBlobSHA(tree, doczConfigFile)
	if configSHA == "" {
		return nil, fmt.Errorf("%s/%s has no %s at HEAD", owner, name, doczConfigFile)
	}

	snap := &ingest.RepoSnapshot{HeadSHA: headSHA, DefaultBranch: branch}
	if snap.ConfigYAML, err = c.fetchBlob(ctx, owner, name, configSHA); err != nil {
		return nil, fmt.Errorf("fetch %s: %w", doczConfigFile, err)
	}
	docsDir := docsDirHint(snap.ConfigYAML)
	api := apiHint(snap.ConfigYAML)

	// The changelog is opt-in (IMPL-0005): only a repo whose .docz.yaml enables
	// the changelog: block gets its file fetched, so a repo that never opts in
	// costs zero extra requests and serves no changelog.
	if enabled, clPath := changelogHint(snap.ConfigYAML); enabled {
		if clSHA := findBlobSHA(tree, clPath); clSHA != "" {
			if snap.ChangelogMD, err = c.fetchBlob(ctx, owner, name, clSHA); err != nil {
				return nil, fmt.Errorf("fetch %s: %w", clPath, err)
			}
			snap.ChangelogSHA = clSHA
		}
	}
	// The repo home (DESIGN-0003) needs docs_dir before ingest parses the
	// config, so a fetch-scoped hint parse targets the exact path in the
	// already-listed tree — at most one extra blob request.
	indexPath := landingPath(docsDir, &api)
	if indexSHA := findBlobSHA(tree, indexPath); indexSHA != "" {
		if snap.IndexMD, err = c.fetchBlob(ctx, owner, name, indexSHA); err != nil {
			return nil, fmt.Errorf("fetch %s: %w", indexPath, err)
		}
		snap.IndexSHA = indexSHA
	}
	if snap.Blobs, err = c.fetchDocBlobs(ctx, owner, name, classifyTree(tree, docsDir, api.enabled)); err != nil {
		return nil, err
	}
	// Dormancy gates the additional_docs fetch — a disabled block's entries
	// are never requested.
	if !api.enabled {
		return snap, nil
	}
	extra, err := c.fetchAdditionalDocs(ctx, owner, name, tree, api.additionalDocs)
	if err != nil {
		return nil, err
	}
	snap.Blobs = append(snap.Blobs, extra...)
	return snap, nil
}

// resolveHead resolves the default branch, its HEAD sha, and the full
// recursive tree for one repo.
func (c *Client) resolveHead(
	ctx context.Context, owner, name string,
) (branch, headSHA string, tree *github.Tree, err error) {
	repo, _, err := c.gh.Repositories.Get(ctx, owner, name)
	if err != nil {
		return "", "", nil, fmt.Errorf("get repo %s/%s: %w", owner, name, err)
	}
	branch = repo.GetDefaultBranch()

	ref, _, err := c.gh.Git.GetRef(ctx, owner, name, "heads/"+branch)
	if err != nil {
		return "", "", nil, fmt.Errorf("get ref heads/%s: %w", branch, err)
	}
	headSHA = ref.GetObject().GetSHA()

	tree, _, err = c.gh.Git.GetTree(ctx, owner, name, headSHA, true)
	if err != nil {
		return "", "", nil, fmt.Errorf("get tree %s: %w", headSHA, err)
	}
	if tree.GetTruncated() {
		return "", "", nil, fmt.Errorf(
			"tree for %s/%s at %s is truncated; shallow-clone path not implemented", owner, name, headSHA)
	}
	return branch, headSHA, tree, nil
}

// landingPath returns the path the repo-home cache is sourced from: with a
// dormant api: block, the DESIGN-0003 docs_dir/index.md; with an enabled one,
// the hint's landing_page (DESIGN-0004), falling back to docz's own
// <docs_dir>/index.md backfill.
func landingPath(docsDir string, api *apiHints) string {
	if !api.enabled {
		return path.Join(docsDir, doczcfg.WikiIndexName)
	}
	if api.landingPage != "" {
		return api.landingPage
	}
	return path.Join(docsDir, doczcfg.APILandingFileName)
}

// fetchAdditionalDocs resolves each additional_docs entry against the
// already-listed tree: one blob request per present file, zero for absent
// ones (R10: docz never checked existence; ingest skips and reports).
// Non-markdown entries are skipped without a request — ingest discards them
// anyway (DESIGN-0011 clause 1) and warns from the config, so there is no
// point buffering bytes that will never publish.
func (c *Client) fetchAdditionalDocs(
	ctx context.Context, owner, name string, tree *github.Tree, entries []string,
) ([]ingest.BlobEntry, error) {
	var blobs []ingest.BlobEntry
	for _, entry := range entries {
		if !strings.HasSuffix(entry, ".md") {
			continue
		}
		sha := findBlobSHA(tree, entry)
		if sha == "" {
			continue
		}
		content, err := c.fetchBlob(ctx, owner, name, sha)
		if err != nil {
			return nil, fmt.Errorf("fetch %s: %w", entry, err)
		}
		blobs = append(blobs, ingest.BlobEntry{Path: entry, GitSHA: sha, Content: content})
	}
	return blobs, nil
}

// docsDirHint extracts docs_dir from raw .docz.yaml bytes for path targeting
// only — the authoritative parse and validation stay in ingest's loadConfig,
// so a malformed config falls back to docz's default here and still fails
// ingest there. The default and dialect both come from the pinned docz
// library, keeping the hint drift-free by construction.
func docsDirHint(configYAML []byte) string {
	var cfg struct {
		DocsDir string `yaml:"docs_dir"`
	}
	if err := yaml.Unmarshal(configYAML, &cfg); err == nil && cfg.DocsDir != "" {
		// hintPath keeps the hint aligned with docz's normalization (and the
		// classifier's cleanRepoDir), so "./docs" targets the same tree
		// prefix "docs" does.
		return strings.TrimSuffix(hintPath(cfg.DocsDir), "/")
	}
	return doczcfg.DefaultConfig().DocsDir
}

// changelogHint extracts the changelog: block from raw .docz.yaml bytes for
// path targeting only, on the same terms as docsDirHint: the authoritative
// parse, normalization, and validation stay in ingest's loadConfig, so a
// malformed config yields the docz defaults here and still fails ingest there.
// A dormant or absent block reports enabled=false, which is what keeps the
// fetch opt-in.
func changelogHint(configYAML []byte) (enabled bool, file string) {
	def := doczcfg.DefaultConfig().Changelog

	var cfg struct {
		Changelog struct {
			Enabled bool   `yaml:"enabled"`
			File    string `yaml:"file"`
		} `yaml:"changelog"`
	}
	if err := yaml.Unmarshal(configYAML, &cfg); err != nil {
		return def.Enabled, def.File
	}
	file = strings.TrimPrefix(strings.TrimSpace(cfg.Changelog.File), "./")
	if file == "" {
		file = def.File
	}
	return cfg.Changelog.Enabled, file
}

// apiHints is the fetch-scoped read of the .docz.yaml api: block, on the same
// terms as the other hints: the authoritative parse, normalization, and
// validation stay in ingest's loadConfig; the hint only decides what to
// fetch, so a malformed config yields the docz defaults here (disabled) and
// still fails ingest there. exclude is parsed for hint completeness (it
// mirrors docz's normalization and is the future sha-gated-fetch cost lever,
// DESIGN-0004 OQ-2) but the fetch deliberately does not prune by it —
// exclusion filtering is ingest's, where the authoritative config lives.
type apiHints struct {
	enabled        bool
	landingPage    string
	exclude        []string
	additionalDocs []string
}

// apiHint extracts the api: block from raw .docz.yaml bytes for path
// targeting only — the third hint beside docsDirHint and changelogHint.
func apiHint(configYAML []byte) apiHints {
	var cfg struct {
		API struct {
			Enabled        bool     `yaml:"enabled"`
			LandingPage    string   `yaml:"landing_page"`
			Exclude        []string `yaml:"exclude"`
			AdditionalDocs []string `yaml:"additional_docs"`
		} `yaml:"api"`
	}
	if err := yaml.Unmarshal(configYAML, &cfg); err != nil {
		return apiHints{} // the docz default: a dormant block
	}

	h := apiHints{enabled: cfg.API.Enabled, landingPage: hintPath(cfg.API.LandingPage)}
	for _, entry := range cfg.API.Exclude {
		// Exclude entries name directories: a single trailing "/" means the
		// same thing without it (docz's normalizeExcludePrefix).
		if e := strings.TrimSuffix(hintPath(entry), "/"); e != "" {
			h.exclude = append(h.exclude, e)
		}
	}
	for _, entry := range cfg.API.AdditionalDocs {
		if e := hintPath(entry); e != "" {
			h.additionalDocs = append(h.additionalDocs, e)
		}
	}
	return h
}

// hintPath mirrors docz's normalizeRepoPath for hint purposes: surrounding
// whitespace trimmed and every leading "./" stripped.
func hintPath(value string) string {
	value = strings.TrimSpace(value)
	for strings.HasPrefix(value, "./") {
		value = value[len("./"):]
	}
	return value
}

// findBlobSHA returns the sha of the blob at exactly path p in tree, or ""
// when no such blob exists.
func findBlobSHA(tree *github.Tree, p string) string {
	for _, e := range tree.Entries {
		if e.GetType() == "blob" && e.GetPath() == p {
			return e.GetSHA()
		}
	}
	return ""
}

// classifyTree collects the doc blobs matching the docz filename convention
// and — only when the api: block is enabled — every other .md under docsDir
// (the page candidates, DESIGN-0004; exclusion pruning stays in ingest). With
// a dormant block the keep-set is exactly the pre-api one. The .docz.yaml,
// changelog, landing-page, and additional_docs files are config-driven, so
// they are resolved by exact path against the same tree rather than
// recognized here.
func classifyTree(tree *github.Tree, docsDir string, apiEnabled bool) (docs []*github.TreeEntry) {
	pagePrefix := docsDir + "/"
	for _, e := range tree.Entries {
		if e.GetType() != "blob" {
			continue
		}
		switch p := e.GetPath(); {
		case doczdoc.IsDoczFile(path.Base(p)):
			docs = append(docs, e)
		case apiEnabled && strings.HasPrefix(p, pagePrefix) && strings.HasSuffix(p, ".md"):
			docs = append(docs, e)
		}
	}
	return docs
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
