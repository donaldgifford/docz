package config_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/pkg/doczcore/config"
)

// TestJSONTags_MirrorYAML walks every struct reachable from Config and
// fails on any exported field whose json tag is missing or diverges from
// its yaml tag (name or omitempty). Without a json tag, json.Marshal
// emits the Go field name — which is what consumers saw in docz-api's
// config_snapshot before issue #89: {"DocsDir": ..., "API":
// {"LandingPage": ...}} instead of the .docz.yaml spellings docz-site
// reads. The walk is recursive so a future config block cannot ship
// untagged (DESIGN-0008 R11).
func TestJSONTags_MirrorYAML(t *testing.T) {
	t.Parallel()
	checkJSONTags(t, reflect.TypeFor[config.Config](), map[reflect.Type]bool{})
}

func checkJSONTags(t *testing.T, typ reflect.Type, seen map[reflect.Type]bool) {
	t.Helper()
	for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice ||
		typ.Kind() == reflect.Map {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct || seen[typ] {
		return
	}
	seen[typ] = true

	for f := range typ.Fields() {
		if !f.IsExported() {
			continue
		}
		yamlName, yamlOmit := splitTag(f.Tag.Get("yaml"))
		jsonName, jsonOmit := splitTag(f.Tag.Get("json"))
		switch {
		case yamlName == "":
			t.Errorf("%s.%s: missing yaml tag", typ.Name(), f.Name)
		case jsonName == "":
			t.Errorf(
				"%s.%s: missing json tag — json.Marshal would emit the Go field name",
				typ.Name(), f.Name,
			)
		case jsonName != yamlName:
			t.Errorf(
				"%s.%s: json tag %q does not match yaml tag %q",
				typ.Name(), f.Name, jsonName, yamlName,
			)
		case jsonOmit != yamlOmit:
			t.Errorf(
				"%s.%s: omitempty mismatch: yaml=%t json=%t",
				typ.Name(), f.Name, yamlOmit, jsonOmit,
			)
		}
		checkJSONTags(t, f.Type, seen)
	}
}

func splitTag(tag string) (name string, omitempty bool) {
	parts := strings.Split(tag, ",")
	return parts[0], slices.Contains(parts[1:], "omitempty")
}

// TestConfigJSON_MarshaledShape pins the exact serialized shape of a
// fully-populated Config: key spellings, nesting, and field order. This
// is the config_snapshot surface docz-api serves and docz-site reads
// (DESIGN-0008 R11) — a key rename here is a consumer-breaking change.
func TestConfigJSON_MarshaledShape(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		DocsDir: "docs",
		Types: map[string]config.TypeConfig{
			"frameworks": {
				Enabled:     true,
				Dir:         "frameworks",
				Template:    "docs/templates/frameworks.md",
				IDPrefix:    "FW",
				IDWidth:     4,
				Statuses:    []string{"Draft", "Adopted"},
				StatusField: "status",
				PluralLabel: "Frameworks",
				Aliases:     []string{"fw"},
			},
		},
		Index:  config.IndexConfig{AutoUpdate: true, PreserveHeader: true},
		Author: config.AuthorConfig{FromGit: true, Default: "Jane Doe"},
		Wiki: config.WikiConfig{
			AutoUpdate:         true,
			MkDocsPath:         "mkdocs.yml",
			Plugins:            []string{"search"},
			MarkdownExtensions: []string{"admonition"},
			Exclude:            []string{"templates"},
			NavTitles:          map[string]string{"frameworks": "Frameworks"},
			DocsDir:            "docs",
			RepoURL:            "https://github.com/example/repo",
			SiteURL:            "https://example.github.io/repo",
			Theme:              "material",
		},
		TOC:       config.TOCConfig{Enabled: true, MinHeadings: 3},
		Changelog: config.ChangelogConfig{Enabled: true, File: "CHANGELOG.md"},
		API: config.APIConfig{
			Enabled:        true,
			LandingPage:    "docs/index.md",
			Exclude:        []string{"examples"},
			AdditionalDocs: []string{"CONTRIBUTING.md"},
		},
	}

	got, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}

	want := `{
  "docs_dir": "docs",
  "types": {
    "frameworks": {
      "enabled": true,
      "dir": "frameworks",
      "template": "docs/templates/frameworks.md",
      "id_prefix": "FW",
      "id_width": 4,
      "statuses": [
        "Draft",
        "Adopted"
      ],
      "status_field": "status",
      "plural_label": "Frameworks",
      "aliases": [
        "fw"
      ]
    }
  },
  "index": {
    "auto_update": true,
    "preserve_header": true
  },
  "author": {
    "from_git": true,
    "default": "Jane Doe"
  },
  "wiki": {
    "auto_update": true,
    "mkdocs_path": "mkdocs.yml",
    "plugins": [
      "search"
    ],
    "markdown_extensions": [
      "admonition"
    ],
    "exclude": [
      "templates"
    ],
    "nav_titles": {
      "frameworks": "Frameworks"
    },
    "docs_dir": "docs",
    "repo_url": "https://github.com/example/repo",
    "site_url": "https://example.github.io/repo",
    "theme": "material"
  },
  "toc": {
    "enabled": true,
    "min_headings": 3
  },
  "changelog": {
    "enabled": true,
    "file": "CHANGELOG.md"
  },
  "api": {
    "enabled": true,
    "landing_page": "docs/index.md",
    "exclude": [
      "examples"
    ],
    "additional_docs": [
      "CONTRIBUTING.md"
    ]
  }
}`
	if string(got) != want {
		t.Errorf("marshaled shape mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestConfigJSON_OmitEmptyParity pins which keys disappear from a
// zero-value marshal: exactly the fields whose yaml tags carry
// omitempty, so the JSON and YAML shapes drop the same optional keys.
func TestConfigJSON_OmitEmptyParity(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   any
		want string
	}{
		{
			name: "TypeConfig drops plural_label and aliases",
			in:   config.TypeConfig{},
			want: `{"enabled":false,"dir":"","template":"","id_prefix":"",` +
				`"id_width":0,"statuses":null,"status_field":""}`,
		},
		{
			name: "WikiConfig drops plugins, markdown_extensions, docs_dir, repo_url, site_url, theme",
			in:   config.WikiConfig{},
			want: `{"auto_update":false,"mkdocs_path":"","exclude":null,"nav_titles":null}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := json.Marshal(tc.in)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("got %s\nwant %s", got, tc.want)
			}
		})
	}
}
