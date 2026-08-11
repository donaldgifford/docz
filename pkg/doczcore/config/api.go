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
