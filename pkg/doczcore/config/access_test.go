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

package config_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

func TestDefaultConfig_AccessIsPublicRead(t *testing.T) {
	t.Parallel()

	got := config.DefaultConfig().Access.Scopes
	if want := []string{config.ScopePublicRead}; !slices.Equal(got, want) {
		t.Errorf("DefaultConfig().Access.Scopes = %q, want %q", got, want)
	}
}

func TestParseBytes_AccessNormalization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		yaml string
		want []string
	}{
		{
			name: "absent block is public:read",
			yaml: "docs_dir: docs\n",
			want: []string{config.ScopePublicRead},
		},
		{
			name: "explicitly empty list is backfilled",
			yaml: "access:\n  scopes: []\n",
			want: []string{config.ScopePublicRead},
		},
		{
			name: "absent scopes key is backfilled",
			yaml: "access: {}\n",
			want: []string{config.ScopePublicRead},
		},
		{
			name: "entries are trimmed and folded to lower case",
			yaml: "access:\n  scopes: [\" Platform-Team:READ \"]\n",
			want: []string{"platform-team:read"},
		},
		{
			name: "duplicates after folding are dropped, order kept",
			yaml: "access:\n  scopes: [team:read, Public:read, TEAM:read, public:read]\n",
			want: []string{"team:read", config.ScopePublicRead},
		},
		{
			name: "blank entries are dropped",
			yaml: "access:\n  scopes: [\"\", \"  \", team:read]\n",
			want: []string{"team:read"},
		},
		{
			name: "a team scope replaces the default rather than joining it",
			yaml: "access:\n  scopes: [platform-team:read]\n",
			want: []string{"platform-team:read"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := config.ParseBytes([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("ParseBytes: %v", err)
			}

			if !slices.Equal(cfg.Access.Scopes, tt.want) {
				t.Errorf("Access.Scopes = %q, want %q", cfg.Access.Scopes, tt.want)
			}
		})
	}
}

func TestValidate_Access(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		yaml    string
		wantErr string // substring of the error; empty means valid
	}{
		{name: "public:read", yaml: "access:\n  scopes: [public:read]\n"},
		{name: "a team audience", yaml: "access:\n  scopes: [platform-team:read, a1:read]\n"},
		{name: "a 63-character audience", yaml: "access:\n  scopes: [" + strings.Repeat("a", 63) + ":read]\n"},
		{
			name:    "write is reserved",
			yaml:    "access:\n  scopes: [impl:write]\n",
			wantErr: `access.scopes[0]: scope "impl:write": action "write" is reserved`,
		},
		{
			name:    "an unknown action",
			yaml:    "access:\n  scopes: [team:admin]\n",
			wantErr: `action "admin" must be "read"`,
		},
		{
			name:    "a bare public names the fix",
			yaml:    "access:\n  scopes: [public]\n",
			wantErr: `scope "public" has no action; write "public:read"`,
		},
		{
			name:    "no colon",
			yaml:    "access:\n  scopes: [team]\n",
			wantErr: `scope "team" is not <audience>:<action>`,
		},
		{
			name:    "an audience starting with a hyphen",
			yaml:    "access:\n  scopes: [-team:read]\n",
			wantErr: `audience "-team" must match`,
		},
		{
			name:    "an audience with an underscore",
			yaml:    "access:\n  scopes: [my_team:read]\n",
			wantErr: `audience "my_team" must match`,
		},
		{
			name:    "a 64-character audience",
			yaml:    "access:\n  scopes: [" + strings.Repeat("a", 64) + ":read]\n",
			wantErr: "must match",
		},
		{
			name:    "an empty audience",
			yaml:    "access:\n  scopes: [\":read\"]\n",
			wantErr: `audience "" must match`,
		},
		{
			name:    "a second colon lands in the action",
			yaml:    "access:\n  scopes: [team:read:extra]\n",
			wantErr: `action "read:extra" must be "read"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := config.ParseBytes([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("ParseBytes: %v", err)
			}

			_, err = cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}

				return
			}

			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() = %v, want an error containing %q", err, tt.wantErr)
			}

			if !errors.Is(err, config.ErrInvalidAccess) {
				t.Errorf("Validate() = %v, want errors.Is ErrInvalidAccess", err)
			}
		})
	}
}

// TestValidate_AccessDuplicateOnHandBuiltConfig pins that Validate judges a
// Config a consumer built in Go, where no normaliser ran, as strictly as a
// loaded one.
func TestValidate_AccessDuplicateOnHandBuiltConfig(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Access.Scopes = []string{"team:read", "team:read"}

	_, err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), `access.scopes[1] "team:read" is listed twice`) {
		t.Fatalf("Validate() = %v, want the duplicate named", err)
	}

	if !errors.Is(err, config.ErrInvalidAccess) {
		t.Errorf("Validate() = %v, want errors.Is ErrInvalidAccess", err)
	}
}

func TestValidate_AccessEmptyHandBuiltListIsValid(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Access.Scopes = nil

	if _, err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil: an empty list means the default and is the normaliser's to fill", err)
	}
}

func TestValidateScope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scope string
		ok    bool
	}{
		{scope: "public:read", ok: true},
		{scope: "confluence:write", ok: false},
		{scope: "Team:read", ok: false},
		{scope: "team:Read", ok: false},
		{scope: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.scope, func(t *testing.T) {
			t.Parallel()

			err := config.ValidateScope(tt.scope)
			if (err == nil) != tt.ok {
				t.Errorf("ValidateScope(%q) = %v, want ok=%v", tt.scope, err, tt.ok)
			}
		})
	}
}
