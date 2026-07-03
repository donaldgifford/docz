// Package ingest runs the synchronous fetch → parse → map → upsert pipeline
// that turns a repo's docz content at HEAD into rows in the store.
//
// It defines the RepoFetcher boundary (implemented by internal/githubapp) so
// the pipeline is unit-testable with an in-memory fake, and owns the parse and
// row-mapping steps built on the pinned docz parsing library.
package ingest

import "context"

// RepoFetcher retrieves a snapshot of one repo's docz content at HEAD. The
// concrete implementation lives in internal/githubapp; tests use an in-memory
// fake. The snapshot carries raw bytes so ingest never touches the network
// again after Fetch returns.
type RepoFetcher interface {
	Fetch(ctx context.Context, owner, name string) (*RepoSnapshot, error)
}

// RepoSnapshot is the raw, fetched state of one repo at a single HEAD sha. All
// content is plain bytes; no docz-library types cross this boundary.
type RepoSnapshot struct {
	// HeadSHA is the resolved HEAD commit sha of the default branch.
	HeadSHA string
	// DefaultBranch is the repo's default branch name (e.g. "main").
	DefaultBranch string
	// ConfigYAML is the raw bytes of .docz.yaml fetched from HEAD.
	ConfigYAML []byte
	// ChangelogMD is the raw bytes of a root CHANGELOG.md, or nil if absent.
	ChangelogMD []byte
	// ChangelogSHA is the git blob sha of CHANGELOG.md, or "" if absent.
	ChangelogSHA string
	// Blobs is every matched doc file fetched under docs_dir/<type.dir>/.
	Blobs []BlobEntry
}

// BlobEntry is one fetched file blob from the Git Trees API.
type BlobEntry struct {
	// Path is the repo-relative path (e.g. "docs/frameworks/0001-intro.md").
	Path string
	// GitSHA is the blob sha, stored for stable reference.
	GitSHA string
	// Content is the decoded (base64-decoded) file bytes.
	Content []byte
}
