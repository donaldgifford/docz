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
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// syncBase is a valid, enabled sync.confluence block each case breaks once.
const syncBase = `api:
  enabled: true
sync:
  confluence:
    enabled: true
    site: https://example.atlassian.net
    space: DOCZ
    parent: docz
`

// syncWith returns syncBase with each key the override sets replaced by the
// override's line, since yaml rejects a key defined twice.
func syncWith(override string) string {
	var b strings.Builder

	for line := range strings.SplitAfterSeq(syncBase, "\n") {
		key, _, ok := strings.Cut(line, ":")
		if ok && strings.HasPrefix(key, "    ") && strings.Contains(override, key+":") {
			continue
		}
		b.WriteString(line)
	}

	return b.String() + override
}

func TestLoad_SyncBlockNormalization(t *testing.T) {
	t.Parallel()

	cfg, err := config.ParseBytes([]byte(`sync:
  confluence:
    site: " https://example.atlassian.net/ "
    exclude: [archive/, ./templates/]
    mermaid:
      viewer: ""
`))
	if err != nil {
		t.Fatal(err)
	}

	got := cfg.Sync.Confluence
	if got.Site != "https://example.atlassian.net" {
		t.Errorf("Site = %q, want the trailing slash and spaces trimmed", got.Site)
	}
	if strings.Join(got.Exclude, ",") != "archive,templates" {
		t.Errorf("Exclude = %q, want [archive templates]", got.Exclude)
	}
	if got.Mermaid.Viewer != config.MermaidViewerAuto {
		t.Errorf("Mermaid.Viewer = %q, want %q", got.Mermaid.Viewer, config.MermaidViewerAuto)
	}
}

func TestLoad_SyncLayoutDefaults(t *testing.T) {
	t.Parallel()

	// The Phase A shape: parent set, no layout. It loads as the folder
	// layout with parent as the page the folder sits under.
	cfg, err := config.ParseBytes([]byte(syncBase + "    folder: \" docs \"\n"))
	if err != nil {
		t.Fatal(err)
	}

	got := cfg.Sync.Confluence
	if got.Layout != config.LayoutFolder {
		t.Errorf("Layout = %q, want %q", got.Layout, config.LayoutFolder)
	}
	if got.Parent != "docz" || got.Folder != "docs" {
		t.Errorf("Parent, Folder = %q, %q; want docz, docs", got.Parent, got.Folder)
	}
	if _, err := cfg.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}

	if d := config.DefaultConfig(); d.Sync.Confluence.Layout != config.LayoutFolder {
		t.Errorf("DefaultConfig layout = %q, want %q", d.Sync.Confluence.Layout, config.LayoutFolder)
	}
}

func TestValidate_SyncDormantBlockIsNeverJudged(t *testing.T) {
	t.Parallel()

	cfg, err := config.ParseBytes([]byte(`sync:
  confluence:
    enabled: false
    site: ftp://nope/path
    layout: tree
    folder: "a\tb"
    types: [nonsense]
    exclude: [../escape]
    api_pages: true
    mermaid:
      viewer: bogus
`))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := cfg.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil for a dormant block", err)
	}
}

func TestValidate_Sync(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		yaml    string // appended under sync.confluence, overriding syncBase
		wantErr string // "" means valid
	}{
		{name: "base block is valid"},
		{name: "http site", yaml: "    site: http://example.atlassian.net\n", wantErr: "sync.confluence.site"},
		{name: "site with a path", yaml: "    site: https://example.atlassian.net/wiki\n", wantErr: "no path"},
		{name: "site with userinfo", yaml: "    site: https://u:p@example.atlassian.net\n", wantErr: "sync.confluence.site"},
		{name: "site with a query", yaml: "    site: https://example.atlassian.net?x=1\n", wantErr: "sync.confluence.site"},
		{name: "empty site", yaml: "    site: \"\"\n", wantErr: "sync.confluence.site"},
		{name: "empty space", yaml: "    space: \" \"\n", wantErr: "sync.confluence.space"},
		{name: "empty parent in the folder layout", yaml: "    parent: \"\"\n"},
		{name: "folder layout spelled out", yaml: "    layout: Folder\n    folder: Platform docs\n"},
		{name: "page layout", yaml: "    layout: page\n"},
		{name: "empty parent in the page layout", yaml: "    layout: page\n    parent: \"\"\n", wantErr: "sync.confluence.parent"},
		{name: "folder in the page layout", yaml: "    layout: page\n    folder: docz\n", wantErr: "sync.confluence.folder"},
		{name: "unknown layout", yaml: "    layout: tree\n", wantErr: "sync.confluence.layout"},
		{name: "folder with a control character", yaml: "    folder: \"do\\tcz\"\n", wantErr: "control character"},
		{name: "folder too long", yaml: "    folder: " + strings.Repeat("x", 256) + "\n", wantErr: "longer than 255"},
		{name: "types by alias and prefix", yaml: "    types: [inv, DESIGN]\n"},
		{name: "unknown type", yaml: "    types: [nope]\n", wantErr: "sync.confluence.types[0]"},
		{name: "disabled built-in", yaml: "    types: [runbook]\n", wantErr: "not an enabled type"},
		{name: "exclude escapes", yaml: "    exclude: [../x]\n", wantErr: "sync.confluence.exclude[0]"},
		{
			name: "explicit viewer key",
			yaml: "    mermaid:\n      viewer: 23392b90-4271-4239-98ca-a3e96c663cbb/" +
				"63d4d207-ac2f-4273-865c-0240d37f044a/static/mermaid-diagram\n",
		},
		{name: "viewer off", yaml: "    mermaid:\n      viewer: \"off\"\n"},
		{name: "bad viewer", yaml: "    mermaid:\n      viewer: mermaid\n", wantErr: "mermaid.viewer"},
		{name: "api_pages with api on", yaml: "    api_pages: true\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := config.ParseBytes([]byte(syncWith(tt.yaml)))
			if err != nil {
				t.Fatal(err)
			}

			_, err = cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}

			if !errors.Is(err, config.ErrInvalidSync) || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() = %v, want ErrInvalidSync naming %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidate_SyncAPIPagesRequiresAPI(t *testing.T) {
	t.Parallel()

	cfg, err := config.ParseBytes([]byte(`sync:
  confluence:
    enabled: true
    site: https://example.atlassian.net
    space: DOCZ
    parent: docz
    api_pages: true
`))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := cfg.Validate(); !errors.Is(err, config.ErrInvalidSync) ||
		!strings.Contains(err.Error(), "api.enabled") {
		t.Fatalf("Validate() = %v, want ErrInvalidSync naming api.enabled", err)
	}
}
