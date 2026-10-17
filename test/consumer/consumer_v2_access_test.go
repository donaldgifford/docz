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

package consumer_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// TestExternalConsumerReadsAccessScopes proves the access: block
// (DESIGN-0023 §7, IMPL-0026 Phase 1) is reachable from outside the module:
// a manifest's scopes parse, fold, and validate through the public surface,
// and the grammar is exported for a consumer that holds scopes of its own.
func TestExternalConsumerReadsAccessScopes(t *testing.T) {
	t.Parallel()

	cfg, err := config.ParseBytes([]byte("access:\n  scopes: [Platform-Team:read]\n"))
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}

	if want := []string{"platform-team:read"}; !slices.Equal(cfg.Access.Scopes, want) {
		t.Errorf("Access.Scopes = %q, want %q", cfg.Access.Scopes, want)
	}

	if _, err := cfg.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}

	cfg.Access.Scopes = []string{"impl:write"}
	if _, err := cfg.Validate(); !errors.Is(err, config.ErrInvalidAccess) {
		t.Errorf("Validate() with a write scope = %v, want ErrInvalidAccess", err)
	}

	if err := config.ValidateScope(config.ScopePublicRead); err != nil {
		t.Errorf("ValidateScope(%q) = %v, want nil", config.ScopePublicRead, err)
	}
}
