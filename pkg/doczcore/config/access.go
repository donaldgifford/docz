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
	"regexp"
	"slices"
	"strings"
)

// Scope grammar (DESIGN-0023 §7). A scope is `<audience>:<action>`: the
// audience names who may act, the action what they may do. A repository
// lists in access.scopes the scopes that unlock it; a principal reading
// through docz-api holds the scopes its groups or its token grant it, and
// the repository is readable when the two sets meet.
const (
	// ScopePublicRead unlocks a repository for every signed-in reader of
	// docz-api. It is the default and the scope every principal holds.
	ScopePublicRead = "public:read"
	// ActionRead is the one action the grammar accepts today.
	ActionRead = "read"
	// ActionWrite is reserved: the grammar names it so that a later design
	// can add it, and rejects it until one does, with ScopeConfluenceWrite
	// the one exception.
	ActionWrite = "write"
	// ScopeConfluenceWrite is the one scope whose action is not read: a
	// repository lists it to let docz-api's Confluence export write its
	// private documentation to the space its sync: block names (DESIGN-0023
	// §7). It is held by the export alone; no route reads it.
	ScopeConfluenceWrite = "confluence:write"
	// AudiencePublic is the audience of ScopePublicRead.
	AudiencePublic = "public"
)

// AccessConfig maps the top-level access: block, who may read the
// repository through docz-api (DESIGN-0023 §7). Unlike the api:, changelog:,
// and sync: blocks it is never dormant: an absent block means
// [ScopePublicRead], so it is always validated.
type AccessConfig struct {
	// Scopes lists the scopes that unlock the repository. A principal that
	// holds any one of them may read it. Folded to lower case at load time;
	// empty or absent means [ScopePublicRead].
	Scopes []string `yaml:"scopes" json:"scopes"`
}

// ErrInvalidAccess is the sentinel wrapped by every access-block validation
// failure. The wrapped message names the offending entry.
var ErrInvalidAccess = errors.New("invalid access config")

// audiencePattern is the shape of a scope's audience: a DNS-label-like
// token of at most 63 characters that cannot start with a hyphen.
var audiencePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// normalizeAccess settles the access block into one spelling. It never
// rejects: Load normalizes, Validate judges. Each entry is trimmed and
// folded to lower case, an empty list is backfilled to [ScopePublicRead],
// and a duplicate after folding is dropped so that `Public:read` beside
// `public:read` is one scope.
func normalizeAccess(cfg *Config) {
	scopes := make([]string, 0, len(cfg.Access.Scopes))
	for _, s := range cfg.Access.Scopes {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" || slices.Contains(scopes, s) {
			continue
		}

		scopes = append(scopes, s)
	}

	if len(scopes) == 0 {
		scopes = append(scopes, ScopePublicRead)
	}

	cfg.Access.Scopes = scopes
}

// validateAccess rejects an access block docz-api could not act on. It
// runs unconditionally, since the block always has a value, and judges a
// hand-built Config as strictly as a loaded one: a duplicate the
// normaliser would have dropped is still an error here.
func (c *Config) validateAccess() error {
	seen := make(map[string]struct{}, len(c.Access.Scopes))
	for i, scope := range c.Access.Scopes {
		if err := ValidateScope(scope); err != nil {
			return fmt.Errorf("%w: access.scopes[%d]: %w", ErrInvalidAccess, i, err)
		}

		if _, dup := seen[scope]; dup {
			return fmt.Errorf("%w: access.scopes[%d] %q is listed twice", ErrInvalidAccess, i, scope)
		}

		seen[scope] = struct{}{}
	}

	return nil
}

// ValidateScope checks one scope against the grammar: `<audience>:<action>`
// with the audience matching ^[a-z0-9][a-z0-9-]{0,62}$ and the action
// ActionRead. ActionWrite is rejected as reserved, except for the literal
// ScopeConfluenceWrite. The error is bare, so a
// caller wraps it with its own sentinel and the key it was found under;
// docz-api's scope map uses it on the principal side as access.scopes
// does on the repository's.
func ValidateScope(scope string) error {
	audience, action, ok := strings.Cut(scope, ":")
	if !ok {
		if scope == AudiencePublic {
			return fmt.Errorf("scope %q has no action; write %q", scope, ScopePublicRead)
		}

		return fmt.Errorf("scope %q is not <audience>:<action>", scope)
	}

	if !audiencePattern.MatchString(audience) {
		return fmt.Errorf("scope %q: audience %q must match %s", scope, audience, audiencePattern)
	}

	switch action {
	case ActionRead:
		return nil
	case ActionWrite:
		if scope == ScopeConfluenceWrite {
			return nil
		}

		return fmt.Errorf("scope %q: action %q is reserved and not yet granted", scope, action)
	default:
		return fmt.Errorf("scope %q: action %q must be %q", scope, action, ActionRead)
	}
}
