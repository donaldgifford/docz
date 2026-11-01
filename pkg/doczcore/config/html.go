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
	"fmt"
	"slices"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/theme"
)

// DefaultTheme is the theme a repository gets when its html: block names
// none, or names one pkg/theme does not have.
const DefaultTheme = "paper"

// HTMLConfig maps the top-level html: block, the settings for the HTML
// docz writes for one repository: `docz export html` and the docz-review
// page (DESIGN-0025 §3, IMPL-0030). docz-site shows many repositories and
// never reads it. Like access: it is never dormant, since an absent block
// means [DefaultTheme].
type HTMLConfig struct {
	// Theme names the pkg/theme theme pages render in. Folded to lower
	// case at load time; empty, absent, or unknown means [DefaultTheme].
	Theme string `yaml:"theme" json:"theme"`

	// unknownTheme holds a name normalizeHTML or Validate replaced with
	// DefaultTheme, so Validate can warn about it however many times it
	// runs.
	unknownTheme string
}

// normalizeHTML settles the html block into one spelling. It never
// rejects: the theme is trimmed and folded to lower case, an empty value
// is backfilled to DefaultTheme, and a name pkg/theme does not have is
// replaced by DefaultTheme and remembered for Validate's warning.
func normalizeHTML(cfg *Config) {
	cfg.HTML.Theme = strings.ToLower(strings.TrimSpace(cfg.HTML.Theme))
	if cfg.HTML.Theme == "" {
		cfg.HTML.Theme = DefaultTheme
	}
	if !slices.Contains(theme.Names(), cfg.HTML.Theme) {
		cfg.HTML.unknownTheme = cfg.HTML.Theme
		cfg.HTML.Theme = DefaultTheme
	}
}

// validateHTML reports an unknown theme as a warning, never an error:
// docz-api ingests every repository's .docz.yaml, and a typo, or a theme
// newer than the deployed server, must not stop a repository ingesting
// (IMPL-0030 open question 3). A hand-built Config naming an unknown
// theme gets DefaultTheme here, as a loaded one does in normalizeHTML.
func (c *Config) validateHTML() []string {
	if c.HTML.Theme != "" && !slices.Contains(theme.Names(), c.HTML.Theme) {
		c.HTML.unknownTheme = c.HTML.Theme
		c.HTML.Theme = DefaultTheme
	}
	if c.HTML.unknownTheme == "" {
		return nil
	}
	return []string{fmt.Sprintf(
		"html.theme %q is not a theme (have %s); using %q",
		c.HTML.unknownTheme, strings.Join(theme.Names(), ", "), DefaultTheme)}
}
