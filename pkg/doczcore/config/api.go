// Copyright 2026 Donald Gifford
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
)

// normalizeAPI settles the api block into the one spelling consumers see,
// after decoding and before validation. It never rejects: Load
// normalizes, Validate judges (DESIGN-0010).
//
// It deliberately does not run the values through path.Clean, which would
// resolve the ".." that validateAPI must still be able to see and reject.
// The one Clean here is on a path docz builds itself, not one a repo
// wrote.
func normalizeAPI(cfg *Config) {
	cfg.API.LandingPage = normalizeRepoPath(cfg.API.LandingPage)

	// Backfilled only while enabled, so a dormant block stays exactly as
	// the repo wrote it and `docz config` does not imply a landing page
	// that nothing will read.
	if cfg.API.Enabled && cfg.API.LandingPage == "" {
		cfg.API.LandingPage = path.Join(cfg.DocsDir, APILandingFileName)
	}

	for i, entry := range cfg.API.Exclude {
		cfg.API.Exclude[i] = normalizeExcludePrefix(entry)
	}

	for i, entry := range cfg.API.AdditionalDocs {
		cfg.API.AdditionalDocs[i] = normalizeRepoPath(entry)
	}
}

// normalizeExcludePrefix reduces an exclude entry to a single spelling.
//
// Exclude is a deny-list, and a consumer matches it by path prefix. If
// "templates" and "templates/" both reached a consumer verbatim, the
// slash-suffixed spelling would be compared as "templates//…" and match
// nothing — an exclusion that fails open, publishing the files the repo
// meant to withhold. Settling it here means every consumer sees one form.
//
// The trailing slash is kept when dropping it would empty the value:
// "/" stays "/" so validateAPI can reject it as absolute rather than as
// an empty entry, which is the more useful message.
func normalizeExcludePrefix(entry string) string {
	entry = normalizeRepoPath(entry)
	if trimmed := strings.TrimSuffix(entry, "/"); trimmed != "" {
		return trimmed
	}
	return entry
}

// ErrInvalidAPIPath is the sentinel wrapped by every api-block path
// validation failure, so a consumer can tell a bad api path from any other
// config problem without matching on error text. Match it with errors.Is;
// the message carries the detail.
//
// One sentinel covers three keys, so unlike ErrInvalidChangelogFile its
// text cannot name the offending key — the wrapped message does, and the
// rendered chain reads
// `invalid api path: api.exclude[1] "../x" must not traverse outside the
// repo root`.
var ErrInvalidAPIPath = errors.New("invalid api path")

// validateAPI rejects an api block a consumer could not safely resolve
// against a git tree (DESIGN-0011).
//
// It returns immediately unless the block is enabled. That is the dormancy
// guarantee this repo already makes for changelog: a repo can commit the
// block today, before docz-api and docz-site understand it, and docz keeps
// loading the config. The paths are judged at the moment they start being
// used.
//
// Validation is path-shape only. It never checks that a file exists — it
// cannot, since docz-api validates a config it fetched from a git tree
// with no checkout. Consumers skip missing files and report them.
func (c *Config) validateAPI() error {
	if !c.API.Enabled {
		return nil
	}

	// Unreachable via Load, which backfills an empty landing page for an
	// enabled block, but reachable for a hand-built Config — the shared
	// helper's "must not be empty" covers it.
	if err := validateRepoRelativeFile("api.landing_page", c.API.LandingPage); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidAPIPath, err)
	}

	for i, entry := range c.API.Exclude {
		// A directory prefix, so "templates" and "templates/" are the same
		// exclusion (Decision 1). Every other rule applies unchanged.
		field := fmt.Sprintf("api.exclude[%d]", i)
		if err := validateRepoRelativeDir(field, entry); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidAPIPath, err)
		}
	}

	if err := c.validateLandingPage(); err != nil {
		return err
	}

	return c.validateAdditionalDocs()
}

// validateLandingPage rejects a landing page the repo has also told docz
// never to publish. exclude is a deny-list and docs_dir/templates is
// excluded unconditionally, so either overlap is a config that contradicts
// itself: the consumer would withhold the file and the repo's front page
// would 404. Saying so at load time beats debugging it in production.
//
// exclude entries are docs_dir-relative while landing_page is
// repo-relative, so the comparison happens in repo-relative terms.
func (c *Config) validateLandingPage() error {
	docsDir := c.normalizedDocsDir()

	if templates := path.Join(docsDir, TemplatesDir); isUnderDir(c.API.LandingPage, templates) {
		return fmt.Errorf("%w: api.landing_page %q is under %q, which is never published",
			ErrInvalidAPIPath, c.API.LandingPage, templates)
	}

	for i, entry := range c.API.Exclude {
		prefix := path.Join(docsDir, strings.TrimSuffix(entry, "/"))
		if isUnderDir(c.API.LandingPage, prefix) {
			return fmt.Errorf("%w: api.landing_page %q is excluded by api.exclude[%d] %q",
				ErrInvalidAPIPath, c.API.LandingPage, i, entry)
		}
	}

	return nil
}

// validateAdditionalDocs applies the shared path rules plus the rules that
// are specific to additional_docs, whose entries are repo-relative rather
// than docs_dir-relative and so can collide with things nothing else in
// the config can.
func (c *Config) validateAdditionalDocs() error {
	docsDir := c.normalizedDocsDir()
	reserved := c.reservedRouteSegments()

	// Keyed by the folded path, because the point is "names the same file",
	// and a consumer on a case-insensitive filesystem reads "README.md" and
	// "readme.md" as one. The verbatim spelling is still what the message
	// reports.
	seen := make(map[string]int, len(c.API.AdditionalDocs))

	for i, entry := range c.API.AdditionalDocs {
		field := fmt.Sprintf("api.additional_docs[%d]", i)
		if err := validateRepoRelativeFile(field, entry); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidAPIPath, err)
		}

		// Each of these would publish one file at two routes: two nav
		// entries, two search hits, one document.
		if first, dup := seen[foldPath(entry)]; dup {
			return fmt.Errorf("%w: %s %q duplicates api.additional_docs[%d]",
				ErrInvalidAPIPath, field, entry, first)
		}
		seen[foldPath(entry)] = i

		if foldPath(entry) == foldPath(c.API.LandingPage) {
			return fmt.Errorf("%w: %s %q also names api.landing_page",
				ErrInvalidAPIPath, field, entry)
		}

		// additional_docs is the escape hatch for what lies outside
		// docs_dir; anything inside is already consumed by convention.
		if isUnderDir(entry, docsDir) {
			return fmt.Errorf(
				"%w: %s %q is already under docs_dir %q, which is consumed automatically",
				ErrInvalidAPIPath, field, entry, docsDir)
		}

		if err := checkReservedSegment(field, entry, reserved); err != nil {
			return err
		}
	}

	return nil
}

// checkReservedSegment rejects an entry whose first path segment is one an
// enabled type already answers to.
//
// This is the one route ambiguity that dropping the /docs/ namespace
// segment leaves open: a repo-root "design/notes.md" addresses as
// /:owner/:repo/design/notes.md, which the router cannot tell from
// /:owner/:repo/:type/:docId. Catching it at load time is the same move
// validateResolution already makes for colliding type tokens.
//
// The percent-decoded spelling is checked too. Git will happily store a
// file literally named "rfc%2Fnotes.md", and a router that decodes before
// matching sees the two-segment form — so the raw path alone is not what
// the route is built from.
func checkReservedSegment(field, entry string, reserved map[string]string) error {
	candidates := []string{entry}
	if decoded, err := url.PathUnescape(entry); err == nil && decoded != entry {
		candidates = append(candidates, decoded)
	}

	for _, candidate := range candidates {
		segment := foldPath(firstPathSegment(candidate))
		if owner, taken := reserved[segment]; taken {
			return fmt.Errorf(
				"%w: %s %q starts with %q, which is reserved by the enabled %q type",
				ErrInvalidAPIPath, field, entry, segment, owner)
		}
	}

	return nil
}

// reservedRouteSegments returns the folded first path segments that
// enabled types answer to in a document route, mapped to the type that
// claims each one.
//
// The claim set is resolveType's, sourced from resolutionTokens so there
// is one definition of it: canonical name, per-type alias, built-in
// registry alias, and id_prefix. A smaller set here would let "inv/x.md"
// or "implementation/x.md" shadow a type route while validating clean. A
// type's directory is claimed on top, since that is the segment the
// docs_dir layout actually produces.
func (c *Config) reservedRouteSegments() map[string]string {
	enabled := c.EnabledTypes()
	segments := make(map[string]string, len(enabled)*4)
	claim := func(token, owner string) {
		if t := foldPath(strings.TrimSpace(token)); t != "" {
			segments[t] = owner
		}
	}

	for _, rt := range c.resolutionTokens() {
		claim(rt.token, rt.owner)
	}

	for _, name := range enabled {
		claim(firstPathSegment(c.Types[name].Dir), name)
	}

	return segments
}

// normalizedDocsDir returns docs_dir in the form the api rules compare
// against. Unlike the api paths, docs_dir is not itself validated, so
// path.Clean is what makes "docs/.", "docs//", and "docs/sub/.." compare
// as the "docs" every consumer resolves them to. An empty or "." value
// cleans to ".", which isUnderDir reads as the repo root.
func (c *Config) normalizedDocsDir() string {
	return path.Clean(normalizeRepoPath(c.DocsDir))
}

// isUnderDir reports whether p names dir itself or something inside it,
// comparing folded so the answer holds on a case-insensitive consumer.
// The repo root contains everything.
func isUnderDir(p, dir string) bool {
	if dir == "" || dir == "." {
		return true
	}

	p, dir = foldPath(p), foldPath(dir)

	return p == dir || strings.HasPrefix(p, dir+"/")
}

// foldPath returns p in the form two paths are compared in: lowercased,
// because a consumer on APFS, NTFS, or a case-folding route table treats
// "README.md" and "readme.md" as one file.
//
// Unicode normalization is deliberately not applied — that would need
// golang.org/x/text, and this package is stdlib-only by design. NFC and
// NFD spellings of the same name therefore still compare as distinct; see
// DESIGN-0011's consumer contract.
func foldPath(p string) string {
	return strings.ToLower(p)
}

// firstPathSegment returns everything before the first "/", which for a
// validated repo-relative path is a non-empty directory or file name.
func firstPathSegment(p string) string {
	first, _, _ := strings.Cut(p, "/")
	return first
}
