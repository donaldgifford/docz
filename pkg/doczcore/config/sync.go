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
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Mermaid viewer settings accepted by MermaidSyncConfig.Viewer besides an
// explicit extension key.
const (
	// MermaidViewerAuto renders mermaid through the built-in default key of
	// the Atlassian Labs Mermaid diagrams viewer.
	MermaidViewerAuto = "auto"
	// MermaidViewerOff renders mermaid as a plain code panel, for a site
	// without the viewer installed.
	MermaidViewerOff = "off"
)

// Layouts accepted by ConfluenceSyncConfig.Layout (DESIGN-0021 §2).
const (
	// LayoutFolder puts the repository's pages in a Confluence folder named
	// after it, every title prefixed with the folder's name, so several
	// repositories can share a space. The default.
	LayoutFolder = "folder"
	// LayoutPage is the Phase A tree: everything under the parent page,
	// titles unprefixed. For a person exporting into a space they own.
	LayoutPage = "page"
)

// SyncConfig maps the opt-in sync: block, the external services a
// repository's documents are exported to (DESIGN-0020). Confluence is the
// only target today.
type SyncConfig struct {
	Confluence ConfluenceSyncConfig `yaml:"confluence" json:"confluence"`
}

// ConfluenceSyncConfig is the sync.confluence block: which Confluence Cloud
// site, space, and parent page `docz export confluence` writes to.
//
// Like the api: and changelog: blocks it is dormant until Enabled, so
// nothing in it is validated and nothing reads it. Credentials are never
// here: ATLASSIAN_EMAIL and ATLASSIAN_API_TOKEN come from the environment.
type ConfluenceSyncConfig struct {
	// Enabled opts the repository in. Default false.
	Enabled bool `yaml:"enabled" json:"enabled"`
	// Site is the Cloud site, e.g. https://example.atlassian.net. Not a
	// secret; the cloud id and page URLs derive from it.
	Site string `yaml:"site" json:"site"`
	// Space is the key of the space pages are written to.
	Space string `yaml:"space" json:"space"`
	// Layout is LayoutFolder (the default) or LayoutPage.
	Layout string `yaml:"layout" json:"layout"`
	// Folder is the title of the repository's folder in the folder layout.
	// Empty means the repository's name, which the exporter supplies.
	Folder string `yaml:"folder,omitempty" json:"folder,omitempty"`
	// Parent is the title of the page the tree sits beneath: the root page
	// in the page layout, where it is required, and the page the folder
	// sits under in the folder layout, where it is optional.
	Parent string `yaml:"parent" json:"parent"`
	// Types narrows the export to these types. Empty means every enabled
	// type.
	Types []string `yaml:"types" json:"types"`
	// Exclude lists path prefixes under docs_dir that are never exported,
	// with the same spelling rules as api.exclude.
	Exclude []string `yaml:"exclude" json:"exclude"`
	// APIPages also exports the api: block's landing page (as the parent
	// page's body) and its additional docs. Requires api.enabled.
	APIPages bool `yaml:"api_pages" json:"api_pages"`
	// Mermaid controls how mermaid fences render.
	Mermaid MermaidSyncConfig `yaml:"mermaid" json:"mermaid"`
}

// MermaidSyncConfig is the sync.confluence.mermaid block.
type MermaidSyncConfig struct {
	// Viewer is MermaidViewerAuto, MermaidViewerOff, or a Forge extension
	// key of the form <app-id>/<env-id>/static/<module>.
	Viewer string `yaml:"viewer" json:"viewer"`
}

// ErrInvalidSync is the sentinel wrapped by every sync-block validation
// failure. The wrapped message names the offending key.
var ErrInvalidSync = errors.New("invalid sync config")

// viewerKeyPattern is the shape of a Forge extension key.
var viewerKeyPattern = regexp.MustCompile(`^[0-9a-f-]{36}/[0-9a-f-]{36}/static/[a-z0-9-]+$`)

// normalizeSync settles the sync block into one spelling. It never
// rejects: Load normalizes, Validate judges.
func normalizeSync(cfg *Config) {
	c := &cfg.Sync.Confluence
	c.Site = strings.TrimSuffix(strings.TrimSpace(c.Site), "/")
	c.Layout = strings.ToLower(strings.TrimSpace(c.Layout))
	c.Folder = strings.TrimSpace(c.Folder)

	if c.Layout == "" {
		c.Layout = LayoutFolder
	}

	for i, entry := range c.Exclude {
		c.Exclude[i] = normalizeExcludePrefix(entry)
	}

	if strings.TrimSpace(c.Mermaid.Viewer) == "" {
		c.Mermaid.Viewer = MermaidViewerAuto
	}
}

// validateSync rejects an enabled sync.confluence block that an export
// could not act on. A dormant block is never judged, so a repository can
// commit it before turning it on.
func (c *Config) validateSync() error {
	s := &c.Sync.Confluence
	if !s.Enabled {
		return nil
	}

	if err := validateSyncSite(s.Site); err != nil {
		return err
	}

	if strings.TrimSpace(s.Space) == "" {
		return fmt.Errorf("%w: sync.confluence.space must not be empty", ErrInvalidSync)
	}

	if err := validateSyncLayout(s); err != nil {
		return err
	}

	if err := c.validateSyncTypes(); err != nil {
		return err
	}

	for i, entry := range s.Exclude {
		if err := validateRepoRelativeDir(fmt.Sprintf("sync.confluence.exclude[%d]", i), entry); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidSync, err)
		}
	}

	if s.APIPages && !c.API.Enabled {
		return fmt.Errorf("%w: sync.confluence.api_pages requires api.enabled", ErrInvalidSync)
	}

	switch v := s.Mermaid.Viewer; {
	case v == MermaidViewerAuto, v == MermaidViewerOff, viewerKeyPattern.MatchString(v):
	default:
		return fmt.Errorf("%w: sync.confluence.mermaid.viewer %q must be %q, %q, or an extension key",
			ErrInvalidSync, v, MermaidViewerAuto, MermaidViewerOff)
	}

	return nil
}

// maxFolderTitle is the longest folder title Confluence accepts.
const maxFolderTitle = 255

// validateSyncLayout checks the layout and the keys that depend on it: the
// page layout needs a parent page and has no folder, and a folder title
// must be one Confluence can hold.
func validateSyncLayout(s *ConfluenceSyncConfig) error {
	switch s.Layout {
	case LayoutPage:
		if strings.TrimSpace(s.Parent) == "" {
			return fmt.Errorf("%w: sync.confluence.parent must not be empty in the page layout", ErrInvalidSync)
		}

		if s.Folder != "" {
			return fmt.Errorf("%w: sync.confluence.folder must be empty in the page layout", ErrInvalidSync)
		}
	case LayoutFolder:
		if err := validateFolderTitle(s.Folder); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%w: sync.confluence.layout %q must be %q or %q",
			ErrInvalidSync, s.Layout, LayoutFolder, LayoutPage)
	}

	return nil
}

// validateFolderTitle rejects a folder title Confluence would refuse or
// that would read differently from what was written. Empty is allowed: the
// exporter names the folder after the repository.
func validateFolderTitle(title string) error {
	if title != strings.TrimSpace(title) {
		return fmt.Errorf("%w: sync.confluence.folder %q has leading or trailing space", ErrInvalidSync, title)
	}

	if utf8.RuneCountInString(title) > maxFolderTitle {
		return fmt.Errorf("%w: sync.confluence.folder is longer than %d characters", ErrInvalidSync, maxFolderTitle)
	}

	if strings.ContainsFunc(title, unicode.IsControl) {
		return fmt.Errorf("%w: sync.confluence.folder %q contains a control character", ErrInvalidSync, title)
	}

	return nil
}

// validateSyncSite requires an https URL with a host and nothing else.
func validateSyncSite(site string) error {
	u, err := url.Parse(site)
	if err != nil || site == "" {
		return fmt.Errorf("%w: sync.confluence.site %q is not a URL", ErrInvalidSync, site)
	}

	if u.Scheme != "https" || u.Host == "" || u.User != nil ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("%w: sync.confluence.site %q must be https://<host> with no path",
			ErrInvalidSync, site)
	}

	return nil
}

// validateSyncTypes requires each types entry to resolve to an enabled type.
func (c *Config) validateSyncTypes() error {
	for i, token := range c.Sync.Confluence.Types {
		name, ok := c.resolveType(token)
		if !ok || !c.Types[name].Enabled {
			return fmt.Errorf("%w: sync.confluence.types[%d] %q is not an enabled type (enabled: %s)",
				ErrInvalidSync, i, token, strings.Join(c.EnabledTypes(), ", "))
		}
	}

	return nil
}
